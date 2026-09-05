package lifecycle

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/probe"
)

func awg2InterfaceName(section string) string {
	return amnezia.AWG2InterfaceName(section)
}

func setupAWG2Interface(section, rawURI string, updateUCI bool) (string, bool, error) {
	params, err := amnezia.ParseAWG2URI(rawURI)
	if err != nil {
		return "", false, err
	}
	ifname := amnezia.AWG2InterfaceName(section)
	// Old urltest/failover index shifts leave pawg* clones with the same peer and
	// the same tunnel IP. Two such ifaces break bind/routing for probes and traffic.
	adopted := reconcileAWG2IfaceName(ifname, params)

	match, up := awg2PeerConfigMatch(ifname, params)
	if match {
		ensureAWG2NetworkUCI(ifname)
		if updateUCI {
			uciSetSectionInterface(section, ifname)
		}
		// Stale handshake: try alternate Host:Port of the same peer, then bounce.
		// Full delete+recreate + uci commit races netifd and can drop the WAN default route.
		if up {
			if fresh, _ := probe.WgHandshakeFresh(ifname, probe.DefaultWGHandshakeMaxAge); !fresh {
				if rotateAWG2Endpoint(ifname) {
					return ifname, true, nil
				}
				_ = bounceAWGInterface(ifname)
			}
		}
		return ifname, adopted, nil
	}

	_ = exec.Command("ip", "link", "del", "dev", ifname).Run()
	if out, err := exec.Command("ip", "link", "add", "dev", ifname, "type", "amneziawg").CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("create amneziawg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if params.MTU != "" {
		_ = exec.Command("ip", "link", "set", "mtu", params.MTU, "dev", ifname).Run()
	}
	_ = exec.Command("ip", "address", "flush", "dev", ifname).Run()
	if out, err := exec.Command("ip", "address", "add", params.Address, "dev", ifname).CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("address add: %w: %s", err, string(out))
	}

	cfgFile, err := writeAWG2Config(params)
	if err != nil {
		return "", false, err
	}
	defer os.Remove(cfgFile)

	if out, err := exec.Command("awg", "setconf", ifname, cfgFile).CombinedOutput(); err != nil {
		return "", false, wrapAWGSetConf(err, out)
	}
	if out, err := exec.Command("ip", "link", "set", "up", "dev", ifname).CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("link up: %w: %s", err, strings.TrimSpace(string(out)))
	}

	ensureAWG2NetworkUCI(ifname)
	if updateUCI {
		uciSetSectionInterface(section, ifname)
	}
	return ifname, true, nil
}

func wrapAWGSetConf(err error, out []byte) error {
	msg := strings.TrimSpace(string(out))
	if strings.Contains(msg, "RandomTrailers") || strings.Contains(msg, "DisableCookies") {
		return fmt.Errorf("awg setconf: need amneziawg-tools and kmod-amneziawg 3.1+: %w: %s", err, msg)
	}
	return fmt.Errorf("awg setconf: %w: %s", err, msg)
}

// reconcileAWG2IfaceName collapses pawg* clones of the same peer onto ifname.
// Prefers the clone with a fresh handshake so a stale canonical name does not win.
func reconcileAWG2IfaceName(ifname string, params amnezia.AWG2Params) bool {
	var matches []string
	for _, iface := range listPawgInterfaces() {
		match, _ := awg2PeerConfigMatch(iface, params)
		if match {
			matches = append(matches, iface)
		}
	}
	if len(matches) == 0 {
		return false
	}

	best := matches[0]
	bestFresh, _ := probe.WgHandshakeFresh(best, probe.DefaultWGHandshakeMaxAge)
	for _, iface := range matches[1:] {
		fresh, _ := probe.WgHandshakeFresh(iface, probe.DefaultWGHandshakeMaxAge)
		if fresh && !bestFresh {
			best = iface
			bestFresh = true
		}
	}

	changed := false
	for _, iface := range matches {
		if iface == best {
			continue
		}
		_ = exec.Command("ip", "link", "del", "dev", iface).Run()
		removeAWG2NetworkUCI(iface)
		changed = true
	}
	if best == ifname {
		return changed
	}

	_ = exec.Command("ip", "link", "set", "dev", best, "down").Run()
	if out, err := exec.Command("ip", "link", "set", "dev", best, "name", ifname).CombinedOutput(); err != nil {
		_ = exec.Command("ip", "link", "del", "dev", best).Run()
		removeAWG2NetworkUCI(best)
		_ = out
		return true
	}
	_ = exec.Command("ip", "link", "set", "dev", ifname, "up").Run()
	removeAWG2NetworkUCI(best)
	ensureAWG2NetworkUCI(ifname)
	return true
}

func listPawgInterfaces() []string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "pawg") {
			out = append(out, name)
		}
	}
	return out
}

func removeAWG2NetworkUCI(ifname string) {
	section := filepath.Base(ifname)
	_ = exec.Command("uci", "-q", "delete", "network."+section).Run()
	if !uciHasChanges("network") {
		return
	}
	_ = exec.Command("uci", "-q", "commit", "network").Run()
}

// awg2PeerConfigMatch reports whether ifname exists with the expected peer public key.
// Endpoint may differ when the same peer is reached via an alternate server IP.
// Handshake freshness is intentionally ignored: stale peers are rotated/bounced, not rebuilt.
func awg2PeerConfigMatch(ifname string, params amnezia.AWG2Params) (match bool, up bool) {
	out, err := exec.Command("ip", "link", "show", "dev", ifname).CombinedOutput()
	if err != nil {
		return false, false
	}
	up = strings.Contains(string(out), "UP")
	show, err := exec.Command("awg", "show", ifname).CombinedOutput()
	if err != nil {
		return false, up
	}
	_, publicKey, ok := parseAWGShowPeer(string(show))
	if !ok {
		return false, up
	}
	if publicKey != params.PublicKey {
		return false, up
	}
	return true, up
}

func writeAWG2Endpoints(ifname string, endpoints []string) {
	if ifname == "" || len(endpoints) == 0 {
		return
	}
	dir := paths.AWG2EndpointsDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, ifname)
	var b strings.Builder
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		b.WriteString(ep)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(path, []byte(b.String()), 0o644)
}

func readAWG2Endpoints(ifname string) []string {
	if ifname == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(paths.AWG2EndpointsDir, ifname))
	if err != nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

// rotateAWG2Endpoint switches the peer to the next Host:Port from the endpoint list.
// Returns true when the endpoint was changed (caller should treat iface as reconfigured).
func rotateAWG2Endpoint(ifname string) bool {
	endpoints := readAWG2Endpoints(ifname)
	if len(endpoints) < 2 {
		return false
	}
	show, err := exec.Command("awg", "show", ifname).CombinedOutput()
	if err != nil {
		return false
	}
	current, publicKey, ok := parseAWGShowPeer(string(show))
	if !ok {
		return false
	}
	next := nextAWG2Endpoint(current, endpoints)
	if next == "" {
		return false
	}
	log.Printf("hybrid-failover: %s rotate endpoint %s -> %s", ifname, current, next)
	if err := setAWG2Endpoint(ifname, publicKey, next); err != nil {
		log.Printf("hybrid-failover: %s set endpoint: %v", ifname, err)
		return false
	}
	if err := bounceAWGInterface(ifname); err != nil {
		log.Printf("hybrid-failover: %s bounce after rotate: %v", ifname, err)
		return true
	}
	return true
}

func nextAWG2Endpoint(current string, endpoints []string) string {
	if len(endpoints) < 2 {
		return ""
	}
	idx := -1
	for i, ep := range endpoints {
		if ep == current {
			idx = i
			break
		}
	}
	next := endpoints[0]
	if idx >= 0 {
		next = endpoints[(idx+1)%len(endpoints)]
	}
	if next == current {
		return ""
	}
	return next
}

func setAWG2Endpoint(ifname, publicKey, endpoint string) error {
	out, err := exec.Command("awg", "set", ifname, "peer", publicKey, "endpoint", endpoint).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// bounceAWGInterface flaps the link to force a new handshake (keeps addresses/config).
func bounceAWGInterface(ifname string) error {
	if ifname == "" {
		return fmt.Errorf("empty iface")
	}
	_ = exec.Command("ip", "link", "set", "dev", ifname, "down").Run()
	out, err := exec.Command("ip", "link", "set", "dev", ifname, "up").CombinedOutput()
	if err != nil {
		return fmt.Errorf("link up %s: %w: %s", ifname, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func parseAWGShowPeer(text string) (endpoint, publicKey string, ok bool) {
	inPeer := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "peer:") {
			inPeer = true
			publicKey = strings.TrimSpace(strings.TrimPrefix(line, "peer:"))
			continue
		}
		if inPeer && strings.HasPrefix(line, "endpoint:") {
			endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
			return endpoint, publicKey, endpoint != "" && publicKey != ""
		}
	}
	return "", "", false
}

func writeAWG2Config(p amnezia.AWG2Params) (string, error) {
	f, err := os.CreateTemp("", "hybrid-failover-awg2.*")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_ = f.Chmod(0o600)

	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\n", p.PrivateKey)
	awgWriteIfSet(&b, "Jc", p.Jc)
	awgWriteIfSet(&b, "Jmin", p.Jmin)
	awgWriteIfSet(&b, "Jmax", p.Jmax)
	awgWriteIfSet(&b, "S1", p.S1)
	awgWriteIfSet(&b, "S2", p.S2)
	awgWriteIfSet(&b, "S3", p.S3)
	awgWriteIfSet(&b, "S4", p.S4)
	awgWriteIfSet(&b, "H1", p.H1)
	awgWriteIfSet(&b, "H2", p.H2)
	awgWriteIfSet(&b, "H3", p.H3)
	awgWriteIfSet(&b, "H4", p.H4)
	awgWriteIfSet(&b, "I1", p.I1)
	awgWriteIfSet(&b, "I2", p.I2)
	awgWriteIfSet(&b, "I3", p.I3)
	awgWriteIfSet(&b, "I4", p.I4)
	awgWriteIfSet(&b, "I5", p.I5)
	awgWriteIfSet(&b, "HeaderProtectionKey", p.HeaderProtectionKey)
	awgWriteIfSet(&b, "ContentPaddingAddition", p.ContentPaddingAddition)
	// RandomTrailers is two-sided: a 3.0 kmod drops lengthened handshake packets.
	awgWriteIfSet(&b, "RandomTrailers", p.RandomTrailers)
	awgWriteIfSet(&b, "DisableCookies", p.DisableCookies)
	awgWriteIfSet(&b, "RekeyAfterTime", p.RekeyAfterTime)
	awgWriteIfSet(&b, "RekeyTimeout", p.RekeyTimeout)
	awgWriteIfSet(&b, "RejectAfterTime", p.RejectAfterTime)
	awgWriteIfSet(&b, "KeepaliveTimeout", p.KeepaliveTimeout)
	awgWriteIfSet(&b, "MaxHandshakeAttempts", p.MaxHandshakeAttempts)

	fmt.Fprintf(&b, "\n[Peer]\nPublicKey = %s\n", p.PublicKey)
	if p.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
	}
	for _, cidr := range strings.Split(p.AllowedIPs, ",") {
		cidr = strings.TrimSpace(cidr)
		if cidr != "" {
			fmt.Fprintf(&b, "AllowedIPs = %s\n", cidr)
		}
	}
	fmt.Fprintf(&b, "Endpoint = %s:%s\n", p.Host, p.Port)
	fmt.Fprintf(&b, "PersistentKeepalive = %s\n", p.PersistentKeepalive)

	if _, err := f.WriteString(b.String()); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func awgWriteIfSet(b *strings.Builder, key, val string) {
	if strings.TrimSpace(val) != "" {
		fmt.Fprintf(b, "%s = %s\n", key, val)
	}
}

func ensureAWG2NetworkUCI(ifname string) {
	section := filepath.Base(ifname)
	_ = exec.Command("uci", "-q", "set", "network."+section+"=interface").Run()
	_ = exec.Command("uci", "-q", "set", "network."+section+".proto=none").Run()
	_ = exec.Command("uci", "-q", "set", "network."+section+".device="+ifname).Run()
	// Commit only when values actually changed. Blind commit reloads netifd and
	// can drop the WAN default route (seen after PPPoE reconnect + watchdog).
	if !uciHasChanges("network") {
		return
	}
	_ = exec.Command("uci", "-q", "commit", "network").Run()
}

func uciSetSectionInterface(section, ifname string) {
	_ = exec.Command("uci", "-q", "set", paths.UCIPackage+"."+section+".interface="+ifname).Run()
	if !uciHasChanges(paths.UCIPackage) {
		return
	}
	_ = exec.Command("uci", "-q", "commit", paths.UCIPackage).Run()
}

func uciHasChanges(pkg string) bool {
	out, err := exec.Command("uci", "-q", "changes", pkg).CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}
