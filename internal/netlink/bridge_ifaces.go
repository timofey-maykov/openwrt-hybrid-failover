package netlink

import (
	"os"
	"path/filepath"
	"strings"
)

// expandSourceIfaces adds bridge member ports when a bridge (e.g. br-lan) is listed.
// Without bridge-nf-call-iptables, WiFi/Ethernet frames may arrive with iifname set to
// the port (phy0-ap0), not the bridge, and tproxy mangle rules would not match.
func expandSourceIfaces(ifaces []string) []string {
	seen := make(map[string]struct{}, len(ifaces))
	out := make([]string, 0, len(ifaces))
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, iface := range ifaces {
		add(iface)
		if !strings.HasPrefix(iface, "br-") {
			continue
		}
		for _, port := range bridgeMemberIfaces(iface) {
			add(port)
		}
	}
	return out
}

func bridgeMemberIfaces(bridge string) []string {
	dir := filepath.Join("/sys/class/net", bridge, "brif")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := strings.TrimSpace(e.Name())
		if name == "" || name == "." || name == ".." {
			continue
		}
		out = append(out, name)
	}
	return out
}
