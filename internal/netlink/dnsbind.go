package netlink

import (
	"fmt"
	"os/exec"
	"strings"
)

const DNSBindCIDR = "127.0.0.42/32"

// EnsureDNSBindAddr adds 127.0.0.42 on lo for the native engine DNS listener.
func EnsureDNSBindAddr() error {
	out, err := exec.Command("ip", "-4", "addr", "show", "dev", "lo").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip addr show lo: %w", err)
	}
	if strings.Contains(string(out), "127.0.0.42") {
		return nil
	}
	out, err = exec.Command("ip", "-4", "addr", "add", DNSBindCIDR, "dev", "lo").CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip addr add %s dev lo: %w: %s", DNSBindCIDR, err, strings.TrimSpace(string(out)))
	}
	return nil
}
