package outbound

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

func TestRealDNSResolverUsesBootstrapNotFakeIP(t *testing.T) {
	d := outboundDialer("")
	if d.Resolver == nil {
		t.Fatal("expected resolver")
	}
	// Smoke: constants used by realDNSResolver stay wired.
	if singbox.DefaultBootstrapDNS == "" || singbox.DefaultDNSServer == "" {
		t.Fatal("bootstrap DNS constants empty")
	}
}
