package dnsmasq

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	BackupPath     = "/etc/hybrid-failover/dnsmasq-dhcp.bak"
	DNSUpstream    = "127.0.0.42"
)

const backupPath = BackupPath

// Configure redirects DNS to sing-box when Hybrid Failover is running.
func Configure() error {
	if err := os.MkdirAll("/etc/hybrid-failover", 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		out, err := exec.Command("uci", "export", "dhcp").CombinedOutput()
		if err != nil {
			return fmt.Errorf("uci export dhcp: %w", err)
		}
		if err := os.WriteFile(backupPath, out, 0o600); err != nil {
			return err
		}
	}
	_ = exec.Command("uci", "-q", "delete", "dhcp.@dnsmasq[0].server").Run()
	_ = exec.Command("uci", "set", "dhcp.@dnsmasq[0].noresolv=1").Run()
	_ = exec.Command("uci", "add_list", "dhcp.@dnsmasq[0].server="+DNSUpstream).Run()
	if err := exec.Command("uci", "commit", "dhcp").Run(); err != nil {
		return err
	}
	return exec.Command("/etc/init.d/dnsmasq", "restart").Run()
}

// Restore reverts dnsmasq UCI from backup.
func Restore() error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil
	}
	tmp := RestoreApplyTempPath()
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	defer os.Remove(tmp)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("uci import dhcp < %q && uci commit dhcp", tmp))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore dnsmasq: %w: %s", err, string(out))
	}
	_ = os.Remove(backupPath)
	return exec.Command("/etc/init.d/dnsmasq", "restart").Run()
}

// RestoreApplyTempPath returns the temp file path used during Restore.
func RestoreApplyTempPath() string {
	return backupPath + ".apply"
}
