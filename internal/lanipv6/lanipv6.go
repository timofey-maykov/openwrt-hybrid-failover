package lanipv6

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const BackupPath = "/etc/hybrid-failover/lan-ipv6.bak"

// ShouldDisable reports whether LAN IPv6 must be turned off (tproxy is IPv4-only).
func ShouldDisable(uciPath string) bool {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return true
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return true
	}
	return settings.GetBool("disable_lan_ipv6", true)
}

// ApplyFromUCI disables or restores LAN IPv6 according to settings.
func ApplyFromUCI(uciPath string) error {
	if ShouldDisable(uciPath) {
		return Configure()
	}
	return Restore()
}

// Configure turns off IPv6 router advertisements and DHCPv6 on LAN.
func Configure() error {
	if err := os.MkdirAll("/etc/hybrid-failover", 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(BackupPath); os.IsNotExist(err) {
		if err := writeBackup(); err != nil {
			return err
		}
	}
	if err := setLANIPv6Off(); err != nil {
		return err
	}
	_ = exec.Command("/etc/init.d/odhcpd", "restart").Run()
	return nil
}

// Restore reverts network.lan and dhcp.lan IPv6 options from backup.
func Restore() error {
	data, err := os.ReadFile(BackupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	vals, err := parseBackup(string(data))
	if err != nil {
		return err
	}
	if err := applyBackup(vals); err != nil {
		return err
	}
	_ = os.Remove(BackupPath)
	_ = exec.Command("/etc/init.d/odhcpd", "restart").Run()
	return nil
}

func writeBackup() error {
	vals := map[string]string{
		"network.lan.ipv6": uciGet("network.lan.ipv6"),
		"dhcp.lan.dhcpv6":  uciGet("dhcp.lan.dhcpv6"),
		"dhcp.lan.ra":      uciGet("dhcp.lan.ra"),
	}
	var b strings.Builder
	for k, v := range vals {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	for _, item := range uciGetList("dhcp.lan.ra_flags") {
		fmt.Fprintf(&b, "list.dhcp.lan.ra_flags=%s\n", item)
	}
	if v := uciGet("dhcp.lan.ra_slaac"); v != "" {
		fmt.Fprintf(&b, "dhcp.lan.ra_slaac=%s\n", v)
	}
	return os.WriteFile(BackupPath, []byte(b.String()), 0o600)
}

func parseBackup(raw string) (map[string]string, error) {
	lists := make(map[string][]string)
	vals := make(map[string]string)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "list.") {
			rest := strings.TrimPrefix(line, "list.")
			key, val, ok := strings.Cut(rest, "=")
			if !ok {
				return nil, fmt.Errorf("lan-ipv6 backup: bad list line %q", line)
			}
			lists[key] = append(lists[key], val)
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("lan-ipv6 backup: bad line %q", line)
		}
		vals[key] = val
	}
	for k, items := range lists {
		vals[k] = strings.Join(items, "\t")
	}
	return vals, nil
}

func applyBackup(vals map[string]string) error {
	if v, ok := vals["network.lan.ipv6"]; ok && v != "" {
		if err := uciSet("network.lan.ipv6", v); err != nil {
			return err
		}
	} else {
		_ = exec.Command("uci", "-q", "delete", "network.lan.ipv6").Run()
	}
	for _, key := range []string{"dhcp.lan.dhcpv6", "dhcp.lan.ra", "dhcp.lan.ra_slaac"} {
		if v, ok := vals[key]; ok && v != "" {
			if err := uciSet(key, v); err != nil {
				return err
			}
		}
	}
	_ = exec.Command("uci", "-q", "delete", "dhcp.lan.ra_flags").Run()
	if raw, ok := vals["dhcp.lan.ra_flags"]; ok && raw != "" {
		for _, item := range strings.Split(raw, "\t") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			_ = exec.Command("uci", "add_list", "dhcp.lan.ra_flags="+item).Run()
		}
	}
	if err := exec.Command("uci", "commit", "network").Run(); err != nil {
		return err
	}
	return exec.Command("uci", "commit", "dhcp").Run()
}

func setLANIPv6Off() error {
	if err := uciSet("network.lan.ipv6", "0"); err != nil {
		return err
	}
	if err := uciSet("dhcp.lan.dhcpv6", "disabled"); err != nil {
		return err
	}
	if err := uciSet("dhcp.lan.ra", "disabled"); err != nil {
		return err
	}
	_ = exec.Command("uci", "-q", "delete", "dhcp.lan.ra_slaac").Run()
	_ = exec.Command("uci", "-q", "delete", "dhcp.lan.ra_flags").Run()
	if err := exec.Command("uci", "commit", "network").Run(); err != nil {
		return err
	}
	return exec.Command("uci", "commit", "dhcp").Run()
}

func uciGet(key string) string {
	out, err := exec.Command("uci", "-q", "get", key).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func uciGetList(key string) []string {
	out, err := exec.Command("uci", "-q", "get", key).CombinedOutput()
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil
	}
	return strings.Fields(raw)
}

func uciSet(key, value string) error {
	return exec.Command("uci", "set", key+"="+value).Run()
}
