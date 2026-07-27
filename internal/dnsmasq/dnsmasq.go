package dnsmasq

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/lanipv6"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const (
	BackupPath  = "/etc/hybrid-failover/dnsmasq-dhcp.bak"
	DNSUpstream = "127.0.0.42"
	resolvPath  = "/tmp/resolv.conf"
)

const backupPath = BackupPath

// StopService stops dnsmasq so the native engine can bind 127.0.0.42:53.
func StopService() error {
	if err := exec.Command("/etc/init.d/dnsmasq", "stop").Run(); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command("pidof", "dnsmasq").CombinedOutput()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// Configure redirects LAN DNS to the native engine via dnsmasq.
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
	_ = exec.Command("uci", "-q", "delete", "dhcp.@dnsmasq[0].notinterface").Run()
	_ = exec.Command("uci", "add_list", "dhcp.@dnsmasq[0].notinterface=lo").Run()
	_ = exec.Command("uci", "add_list", "dhcp.@dnsmasq[0].server="+DNSUpstream).Run()
	if lanipv6.ShouldDisable(paths.UCIConfig) {
		_ = exec.Command("uci", "set", "dhcp.@dnsmasq[0].filter_aaaa=1").Run()
	} else {
		_ = exec.Command("uci", "-q", "delete", "dhcp.@dnsmasq[0].filter_aaaa").Run()
	}
	if err := exec.Command("uci", "commit", "dhcp").Run(); err != nil {
		return err
	}
	if err := exec.Command("/etc/init.d/dnsmasq", "restart").Run(); err != nil {
		return err
	}
	return ensureLocalResolvPersist()
}

// EnsureLocalResolv points router processes at dnsmasq on the LAN address.
// dnsmasq uses notinterface=lo so 127.0.0.1 is not a valid resolver.
func EnsureLocalResolv() error {
	return ensureLocalResolvOnce()
}

// EnsureLocalResolvIfNeeded rewrites resolv.conf when OpenWrt reset it to 127.0.0.1.
func EnsureLocalResolvIfNeeded() error {
	if !resolvNeedsFix() {
		return nil
	}
	return ensureLocalResolvPersist()
}

// EnsureRunning starts dnsmasq when it is not running (LAN DNS must not stay down after engine restarts).
func EnsureRunning() error {
	out, err := exec.Command("pidof", "dnsmasq").CombinedOutput()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return nil
	}
	return exec.Command("/etc/init.d/dnsmasq", "start").Run()
}

// UsesEngineUpstream reports whether dnsmasq is configured to forward to the native engine DNS.
func UsesEngineUpstream() bool {
	out, err := exec.Command("uci", "-q", "get", "dhcp.@dnsmasq[0].server").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), DNSUpstream)
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
	if err := exec.Command("/etc/init.d/dnsmasq", "restart").Run(); err != nil {
		return err
	}
	return nil
}

func ensureLocalResolvOnce() error {
	lanIP := lanIPAddress()
	if lanIP == "" {
		return fmt.Errorf("dnsmasq: no LAN nameserver address")
	}
	body := "search lan\nnameserver " + lanIP + "\n"
	return os.WriteFile(resolvPath, []byte(body), 0o644)
}

func ensureLocalResolvPersist() error {
	var lastErr error
	for i := 0; i < 8; i++ {
		if err := ensureLocalResolvOnce(); err != nil {
			lastErr = err
		} else if !resolvNeedsFix() {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return ensureLocalResolvOnce()
}

func resolvNeedsFix() bool {
	lanIP := lanIPAddress()
	if lanIP == "" || lanIP == "127.0.0.1" {
		return false
	}
	data, err := os.ReadFile(resolvPath)
	if err != nil {
		return true
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return true
	}
	return !strings.Contains(content, "nameserver "+lanIP)
}

func lanIPAddress() string {
	out, err := exec.Command("uci", "-q", "get", "network.lan.ipaddr").CombinedOutput()
	if err == nil {
		if ip := strings.TrimSpace(string(out)); ip != "" && ip != "127.0.0.1" {
			return ip
		}
	}
	if pkg, err := uci.Load("/etc/config/network"); err == nil {
		if sec := pkg.Section("lan"); sec != nil {
			if ip := strings.TrimSpace(sec.Get("ipaddr", "")); ip != "" && ip != "127.0.0.1" {
				return ip
			}
		}
	}
	return ""
}

// RestoreApplyTempPath returns the temp file path used during Restore.
func RestoreApplyTempPath() string {
	return backupPath + ".apply"
}
