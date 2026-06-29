package lanipv6

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShouldDisableDefaultTrue(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "hybrid-failover")
	if err := os.WriteFile(cfg, []byte("config settings 'settings'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ShouldDisable(cfg) {
		t.Fatal("expected disable_lan_ipv6 default true")
	}
}

func TestShouldDisableExplicitOff(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "hybrid-failover")
	body := "config settings 'settings'\n\toption disable_lan_ipv6 '0'\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if ShouldDisable(cfg) {
		t.Fatal("expected disable_lan_ipv6=0 to allow LAN IPv6")
	}
}

func TestParseBackup(t *testing.T) {
	raw := "network.lan.ipv6=1\ndhcp.lan.dhcpv6=server\ndhcp.lan.ra=server\nlist.dhcp.lan.ra_flags=managed-config\nlist.dhcp.lan.ra_flags=other-config\ndhcp.lan.ra_slaac=1\n"
	vals, err := parseBackup(raw)
	if err != nil {
		t.Fatal(err)
	}
	if vals["network.lan.ipv6"] != "1" {
		t.Fatalf("network.lan.ipv6 = %q", vals["network.lan.ipv6"])
	}
	if !strings.Contains(vals["dhcp.lan.ra_flags"], "managed-config") {
		t.Fatalf("ra_flags = %q", vals["dhcp.lan.ra_flags"])
	}
}
