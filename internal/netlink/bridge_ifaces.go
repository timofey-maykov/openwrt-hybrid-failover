package netlink

import (
	"os"
	"path/filepath"
	"regexp"
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

var dockerBridgeName = regexp.MustCompile(`^(docker0|br-[0-9a-f]{12})$`)

// dockerBridgeIfaces lists the bridges Docker made for its containers: the
// default docker0 and one br-<12 hex> per user network or compose stack. They
// are routed, not bridged, so the bridge itself is the iifname to match and its
// veth ports are not needed.
func dockerBridgeIfaces() []string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !dockerBridgeName.MatchString(name) {
			continue
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", name, "bridge")); err != nil {
			continue
		}
		out = append(out, name)
	}
	return out
}
