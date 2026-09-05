package netlink

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clientrules"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const (
	NFTTable            = "hybrid_failover"
	FWMark              = "0x105"
	RouteTable          = "hybrid_failover"
	ifaceSetName        = "hf_ifaces"
	localv4SetName      = "hf_localv4"
	proxySubnetsSetName = "hf_proxy_subnets"
)

var nftMu sync.Mutex

// Setup applies nft tproxy rules for br-lan + fakeip traffic only (legacy alias).
func Setup() error {
	return ApplyFromUCI(nil)
}

// ApplyFromUCI rebuilds nft rules from UCI (LAN + fakeip scope).
// Serialized: concurrent start/list-update/watchdog Apply races left a table
// with sets but no tproxy chains (add steps hit "No such file" after Teardown).
func ApplyFromUCI(pkg *uci.Package) error {
	nftMu.Lock()
	defer nftMu.Unlock()

	var last error
	for attempt := 0; attempt < 2; attempt++ {
		_ = teardownLocked()
		if err := applyStepsLocked(pkg); err != nil {
			last = err
			continue
		}
		if err := checkLocked(); err != nil {
			last = err
			continue
		}
		return nil
	}
	if last != nil {
		return last
	}
	return fmt.Errorf("nft apply failed")
}

func applyStepsLocked(pkg *uci.Package) error {
	ifaces := []string{"br-lan"}
	if pkg != nil {
		if settings := pkg.Section("settings"); settings != nil {
			if raw := strings.TrimSpace(settings.Get("source_network_interfaces", "")); raw != "" {
				ifaces = strings.Fields(raw)
			}
		}
	}
	ifaces = expandSourceIfaces(ifaces)

	steps := []string{
		"nft add table inet " + NFTTable,
		"nft add set inet " + NFTTable + " " + ifaceSetName + " '{ type ifname; flags interval; }'",
		"nft add set inet " + NFTTable + " " + localv4SetName + " '{ type ipv4_addr; flags interval; }'",
		"nft add element inet " + NFTTable + " " + localv4SetName + " '{ " + localv4Ranges() + " }'",
		"nft add chain inet " + NFTTable + " mangle '{ type filter hook prerouting priority mangle; policy accept; }'",
		"nft add chain inet " + NFTTable + " mangle_output '{ type route hook output priority mangle; policy accept; }'",
		// NAT redirect before tproxy filter: LAN clients that hardcode 8.8.8.8/NextDNS
		// still hit dnsmasq → 127.0.0.42 so FakeIP community domains work (Xbox RL).
		"nft add chain inet " + NFTTable + " dns '{ type nat hook prerouting priority dstnat - 5; policy accept; }'",
		// Drop forwarded DNS that still matches pre-hijack conntrack (UDP "ASSURED" to NextDNS).
		"nft add chain inet " + NFTTable + " dns_forward '{ type filter hook forward priority filter; policy accept; }'",
		"nft add chain inet " + NFTTable + " proxy '{ type filter hook prerouting priority dstnat; policy accept; }'",
	}

	for _, iface := range ifaces {
		iface = strings.TrimSpace(iface)
		if iface == "" {
			continue
		}
		steps = append(steps, "nft add element inet "+NFTTable+" "+ifaceSetName+" '{ "+iface+" }'")
	}

	if pkg != nil {
		rules := clientrules.ListRules(pkg)
		for _, ip := range clientrules.ExcludeIPs(rules) {
			// DNS only here. Mangle return is after FakeIP mark: DHCP clients still
			// resolve via the router and must not blackhole 198.18.0.0/15.
			steps = append(steps, "nft add rule inet "+NFTTable+" dns ip saddr "+quoteIP(ip)+" return")
			steps = append(steps, "nft add rule inet "+NFTTable+" dns_forward ip saddr "+quoteIP(ip)+" return")
		}
		for _, ip := range clientrules.IncludeIPs(rules) {
			steps = append(steps, mangleMarkRule("ip saddr "+quoteIP(ip)))
		}
		// UDP-only full tunnel (e.g. console game traffic) without pulling TCP
		// Xbox Live / Epic HTTPS back into the proxy.
		for _, secName := range pkg.SectionNames("section") {
			sec := pkg.Section(secName)
			if sec == nil {
				continue
			}
			for _, ip := range sec.GetList("udp_routed_ips") {
				ip = strings.TrimSpace(ip)
				if ip == "" {
					continue
				}
				steps = append(steps,
					"nft add rule inet "+NFTTable+" mangle ip saddr "+quoteIP(ip)+
						" ip daddr != @"+localv4SetName+
						" meta l4proto udp th dport != 53 meta mark set "+FWMark,
				)
			}
		}
	}
	steps = append(steps,
		"nft add rule inet "+NFTTable+" dns iifname @"+ifaceSetName+" meta l4proto { tcp, udp } th dport 53 redirect to :53",
		"nft add rule inet "+NFTTable+" dns_forward iifname @"+ifaceSetName+" meta l4proto { tcp, udp } th dport 53 drop",
	)

	if pkg != nil {
		if settings := pkg.Section("settings"); settings != nil && settings.GetBool("disable_quic", false) {
			// Reject QUIC before mark/tproxy so clients fall back to TCP quickly.
			steps = append(steps,
				"nft add rule inet "+NFTTable+" mangle iifname @"+ifaceSetName+" udp dport 443 ip daddr "+singbox.FakeIPInet4Range+" reject",
			)
		}
	}
	steps = append(steps,
		mangleMarkRule("iifname @"+ifaceSetName+" ip daddr "+singbox.FakeIPInet4Range),
	)
	// After FakeIP mark: skip subnet tproxy for consoles / exclude list.
	// FakeIP-marked packets keep their mark and still enter tproxy.
	if pkg != nil {
		rules := clientrules.ListRules(pkg)
		for _, ip := range clientrules.ExcludeIPs(rules) {
			steps = append(steps, mangleReturnRule("ip saddr "+quoteIP(ip)))
		}
		for _, secName := range pkg.SectionNames("section") {
			sec := pkg.Section(secName)
			if sec == nil {
				continue
			}
			for _, ip := range sec.GetList("subnet_bypass_ips") {
				ip = strings.TrimSpace(ip)
				if ip == "" {
					continue
				}
				steps = append(steps, mangleReturnRule("ip saddr "+quoteIP(ip)))
			}
		}
		// Teredo / Xbox NAT traversal must not enter the proxy tunnel.
		steps = append(steps,
			"nft add rule inet "+NFTTable+" mangle meta l4proto udp th dport 3544 return",
		)
	}
	if pkg != nil {
		if cidrs := subnets.NormalizeForNFT(singbox.CollectProxySubnets(pkg)); len(cidrs) > 0 {
			steps = append(steps,
				"nft add set inet "+NFTTable+" "+proxySubnetsSetName+" '{ type ipv4_addr; flags interval; }'",
			)
			const chunkSize = 40
			for i := 0; i < len(cidrs); i += chunkSize {
				end := i + chunkSize
				if end > len(cidrs) {
					end = len(cidrs)
				}
				steps = append(steps,
					"nft add element inet "+NFTTable+" "+proxySubnetsSetName+" '{ "+strings.Join(cidrs[i:end], ", ")+" }'",
				)
			}
			// Do not reject QUIC for proxy_subnets: those CIDRs are huge (CDN/cloud)
			// and ICMP-port-unreachable there freezes apps that stick to HTTP/3.
			// FakeIP QUIC is already rejected above when disable_quic=1.
			steps = append(steps,
				mangleMarkRule("iifname @"+ifaceSetName+" ip daddr @"+proxySubnetsSetName),
				"nft add rule inet "+NFTTable+" mangle_output ip daddr @"+proxySubnetsSetName+" meta l4proto { tcp, udp } meta mark set "+FWMark,
			)
		}
	}
	steps = append(steps,
		"nft add rule inet "+NFTTable+" mangle_output ip daddr @"+localv4SetName+" return",
		"nft add rule inet "+NFTTable+" mangle_output ip daddr "+singbox.FakeIPInet4Range+" meta l4proto { tcp, udp } meta mark set "+FWMark,
		"nft add rule inet "+NFTTable+" proxy meta mark "+FWMark+" meta l4proto tcp tproxy ip to 127.0.0.1:1602 accept",
		"nft add rule inet "+NFTTable+" proxy meta mark "+FWMark+" meta l4proto udp tproxy ip to 127.0.0.1:1602 accept",
	)
	steps = append(steps, ensureIPRulesSteps()...)

	for _, line := range steps {
		out, err := exec.Command("sh", "-c", line).CombinedOutput()
		if err == nil {
			continue
		}
		msg := strings.TrimSpace(string(out))
		// Idempotent re-add only. Never ignore missing table/chain: that is how
		// concurrent Teardown left FakeIP traffic unmarked (apps hang).
		if strings.Contains(msg, "File exists") {
			continue
		}
		return fmt.Errorf("%s: %w: %s", line, err, msg)
	}
	return nil
}

func mangleMarkRule(match string) string {
	return "nft add rule inet " + NFTTable + " mangle " + match + " meta l4proto { tcp, udp } meta mark set " + FWMark
}

func mangleReturnRule(match string) string {
	return "nft add rule inet " + NFTTable + " mangle " + match + " return"
}

func quoteIP(ip string) string {
	ip = strings.TrimSpace(ip)
	if strings.Contains(ip, "/") {
		return ip
	}
	return ip + "/32"
}

func localv4Ranges() string {
	return strings.Join([]string{
		"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16",
		"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24",
		"192.168.0.0/16", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4",
		"240.0.0.0/4",
	}, ", ")
}

func Teardown() error {
	nftMu.Lock()
	defer nftMu.Unlock()
	return teardownLocked()
}

func teardownLocked() error {
	_ = exec.Command("nft", "delete", "table", "inet", NFTTable).Run()
	_ = exec.Command("ip", "rule", "del", "fwmark", FWMark, "table", RouteTable).Run()
	_ = exec.Command("ip", "route", "flush", "table", RouteTable).Run()
	return nil
}

func ensureIPRulesSteps() []string {
	return []string{
		"grep -q '105 " + RouteTable + "' /etc/iproute2/rt_tables 2>/dev/null || echo '105 " + RouteTable + "' >> /etc/iproute2/rt_tables",
		"ip rule add fwmark " + FWMark + " lookup " + RouteTable + " priority 105 2>/dev/null || true",
		"ip route add local default dev lo table " + RouteTable + " 2>/dev/null || true",
	}
}

// EnsureIPRules restores fwmark policy routing required for tproxy (idempotent).
func EnsureIPRules() error {
	nftMu.Lock()
	defer nftMu.Unlock()
	for _, line := range ensureIPRulesSteps() {
		out, err := exec.Command("sh", "-c", line).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "File exists") && !strings.Contains(string(out), "No such file") {
			return fmt.Errorf("%s: %w: %s", line, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func ipRulesOK() bool {
	out, err := exec.Command("ip", "rule", "list").CombinedOutput()
	if err != nil {
		return false
	}
	body := string(out)
	return strings.Contains(body, "fwmark "+FWMark) && strings.Contains(body, "lookup "+RouteTable)
}

func Check() error {
	nftMu.Lock()
	defer nftMu.Unlock()
	return checkLocked()
}

func checkLocked() error {
	out, err := exec.Command("nft", "list", "table", "inet", NFTTable).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft table missing: %w", err)
	}
	body := string(out)
	if !strings.Contains(body, "chain mangle") || !strings.Contains(body, "chain proxy") || !strings.Contains(body, "chain dns") {
		return fmt.Errorf("nft chains incomplete")
	}
	if !strings.Contains(body, "tproxy") {
		return fmt.Errorf("nft rules incomplete")
	}
	if !strings.Contains(body, "redirect to :53") && !strings.Contains(body, "redirect to : 53") {
		return fmt.Errorf("nft dns hijack missing")
	}
	if !strings.Contains(body, "chain dns_forward") {
		return fmt.Errorf("nft dns forward drop missing")
	}
	if !strings.Contains(body, ifaceSetName) {
		return fmt.Errorf("nft interface set missing")
	}
	if !strings.Contains(body, singbox.FakeIPInet4Range) {
		return fmt.Errorf("nft fakeip mark missing")
	}
	if !ipRulesOK() {
		if err := ensureIPRulesLocked(); err != nil {
			return fmt.Errorf("tproxy ip rules missing: %w", err)
		}
		if !ipRulesOK() {
			return fmt.Errorf("tproxy ip rules missing")
		}
	}
	return nil
}

func ensureIPRulesLocked() error {
	for _, line := range ensureIPRulesSteps() {
		out, err := exec.Command("sh", "-c", line).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "File exists") && !strings.Contains(string(out), "No such file") {
			return fmt.Errorf("%s: %w: %s", line, err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
