package dnsmasq_test

import (
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/dnsmasq"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

func TestBackupPathConstant(t *testing.T) {
	if dnsmasq.BackupPath != "/etc/hybrid-failover/dnsmasq-dhcp.bak" {
		t.Fatalf("BackupPath = %q", dnsmasq.BackupPath)
	}
	if !strings.HasPrefix(dnsmasq.BackupPath, "/etc/hybrid-failover/") {
		t.Fatal("backup path should live under /etc/hybrid-failover")
	}
}

func TestDNSUpstreamMatchesSingboxInbound(t *testing.T) {
	if dnsmasq.DNSUpstream != singbox.DNSInboundAddress {
		t.Fatalf("DNSUpstream %q != singbox DNSInboundAddress %q",
			dnsmasq.DNSUpstream, singbox.DNSInboundAddress)
	}
}

func TestRestoreApplyTempPath(t *testing.T) {
	got := dnsmasq.RestoreApplyTempPath()
	want := dnsmasq.BackupPath + ".apply"
	if got != want {
		t.Fatalf("RestoreApplyTempPath() = %q, want %q", got, want)
	}
}
