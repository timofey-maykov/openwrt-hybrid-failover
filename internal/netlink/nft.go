package netlink

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const (
	NFTTable        = "hybrid_failover"
	FWMark          = "0x105"
	RouteTable      = "hybrid_failover"
	ifaceSetName    = "hf_ifaces"
	localv4SetName  = "hf_localv4"
)

// Setup applies nft tproxy rules for br-lan + fakeip traffic only (legacy alias).
func Setup() error {
	return ApplyFromUCI(nil)
}

// ApplyFromUCI rebuilds nft rules from UCI (LAN + fakeip scope).
func ApplyFromUCI(pkg *uci.Package) error {
	_ = Teardown()

	ifaces := []string{"br-lan"}
	if pkg != nil {
		if settings := pkg.Section("settings"); settings != nil {
			if raw := strings.TrimSpace(settings.Get("source_network_interfaces", "")); raw != "" {
				ifaces = strings.Fields(raw)
			}
		}
	}

	steps := []string{
		"nft add table inet " + NFTTable,
		"nft add set inet " + NFTTable + " " + ifaceSetName + " '{ type ifname; flags interval; }'",
		"nft add set inet " + NFTTable + " " + localv4SetName + " '{ type ipv4_addr; flags interval; }'",
		"nft add element inet " + NFTTable + " " + localv4SetName + " '{ " + localv4Ranges() + " }'",
		"nft add chain inet " + NFTTable + " mangle '{ type filter hook prerouting priority mangle; policy accept; }'",
		"nft add chain inet " + NFTTable + " mangle_output '{ type route hook output priority mangle; policy accept; }'",
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
		if settings := pkg.Section("settings"); settings != nil {
			for _, ip := range settings.GetList("exclude_source_ips") {
				steps = append(steps, mangleReturnRule("ip saddr "+quoteIP(ip)))
			}
			for _, ip := range settings.GetList("include_source_ips") {
				steps = append(steps, mangleMarkRule("ip saddr "+quoteIP(ip)))
			}
		}
		for _, name := range pkg.SectionNames("section") {
			sec := pkg.Section(name)
			if sec == nil {
				continue
			}
			for _, ip := range sec.GetList("fully_routed_ips") {
				steps = append(steps, mangleMarkRule("ip saddr "+quoteIP(ip)))
			}
		}
	}

	steps = append(steps,
		mangleMarkRule("iifname @"+ifaceSetName+" ip daddr "+singbox.FakeIPInet4Range),
		"nft add rule inet "+NFTTable+" mangle_output ip daddr @"+localv4SetName+" return",
		"nft add rule inet "+NFTTable+" mangle_output ip daddr "+singbox.FakeIPInet4Range+" meta l4proto { tcp, udp } meta mark set "+FWMark,
		"nft add rule inet "+NFTTable+" proxy meta mark "+FWMark+" meta l4proto tcp tproxy ip to 127.0.0.1:1602 accept",
		"nft add rule inet "+NFTTable+" proxy meta mark "+FWMark+" meta l4proto udp tproxy ip to 127.0.0.1:1602 accept",
		"grep -q '105 "+RouteTable+"' /etc/iproute2/rt_tables 2>/dev/null || echo '105 "+RouteTable+"' >> /etc/iproute2/rt_tables",
		"ip rule add fwmark "+FWMark+" lookup "+RouteTable+" priority 105 2>/dev/null || true",
		"ip route add local default dev lo table "+RouteTable+" 2>/dev/null || true",
	)

	for _, line := range steps {
		out, err := exec.Command("sh", "-c", line).CombinedOutput()
		if err != nil && !strings.Contains(string(out), "File exists") && !strings.Contains(string(out), "No such file") {
			return fmt.Errorf("%s: %w: %s", line, err, strings.TrimSpace(string(out)))
		}
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
	_ = exec.Command("nft", "delete", "table", "inet", NFTTable).Run()
	_ = exec.Command("ip", "rule", "del", "fwmark", FWMark, "table", RouteTable).Run()
	_ = exec.Command("ip", "route", "flush", "table", RouteTable).Run()
	return nil
}

func Check() error {
	out, err := exec.Command("nft", "list", "table", "inet", NFTTable).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft table missing: %w", err)
	}
	body := string(out)
	if !strings.Contains(body, "tproxy") {
		return fmt.Errorf("nft rules incomplete")
	}
	if !strings.Contains(body, ifaceSetName) {
		return fmt.Errorf("nft interface set missing")
	}
	return nil
}
