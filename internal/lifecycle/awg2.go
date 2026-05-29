package lifecycle

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func awg2InterfaceName(section string) string {
	return amnezia.AWG2InterfaceName(section)
}

func setupAWG2Interface(section, rawURI string, updateUCI bool) (string, error) {
	params, err := amnezia.ParseAWG2URI(rawURI)
	if err != nil {
		return "", err
	}
		ifname := amnezia.AWG2InterfaceName(section)

	_ = exec.Command("ip", "link", "del", "dev", ifname).Run()
	if out, err := exec.Command("ip", "link", "add", "dev", ifname, "type", "amneziawg").CombinedOutput(); err != nil {
		return "", fmt.Errorf("create amneziawg: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if params.MTU != "" {
		_ = exec.Command("ip", "link", "set", "mtu", params.MTU, "dev", ifname).Run()
	}
	_ = exec.Command("ip", "address", "flush", "dev", ifname).Run()
	if out, err := exec.Command("ip", "address", "add", params.Address, "dev", ifname).CombinedOutput(); err != nil {
		return "", fmt.Errorf("address add: %w: %s", err, string(out))
	}

	cfgFile, err := writeAWG2Config(params)
	if err != nil {
		return "", err
	}
	defer os.Remove(cfgFile)

	if out, err := exec.Command("awg", "setconf", ifname, cfgFile).CombinedOutput(); err != nil {
		return "", fmt.Errorf("awg setconf: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("ip", "link", "set", "up", "dev", ifname).CombinedOutput(); err != nil {
		return "", fmt.Errorf("link up: %w: %s", err, strings.TrimSpace(string(out)))
	}

	ensureAWG2NetworkUCI(ifname)
	if updateUCI {
		uciSetSectionInterface(section, ifname)
	}
	return ifname, nil
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
	_ = exec.Command("uci", "-q", "commit", "network").Run()
}

func uciSetSectionInterface(section, ifname string) {
	_ = exec.Command("uci", "-q", "set", paths.UCIPackage+"."+section+".interface="+ifname).Run()
	_ = exec.Command("uci", "-q", "commit", paths.UCIPackage).Run()
}
