package partner

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func settings(t *testing.T, body string) *uci.Section {
	t.Helper()
	pkg, err := uci.Parse("config settings 'settings'\n" + body)
	if err != nil {
		t.Fatal(err)
	}
	return pkg.Section("settings")
}

func TestURL(t *testing.T) {
	if got := URL(nil); got != DefaultURL {
		t.Fatalf("nil settings: %q", got)
	}
	if got := URL(settings(t, "")); got != DefaultURL {
		t.Fatalf("unset: %q", got)
	}
	if got := URL(settings(t, "\toption partner_url 'off'\n")); got != "" {
		t.Fatalf("off must disable: %q", got)
	}
	if got := URL(settings(t, "\toption partner_url 'https://example.org/vpn'\n")); got != "https://example.org/vpn" {
		t.Fatalf("custom: %q", got)
	}
}
