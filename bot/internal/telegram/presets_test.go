package telegram

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func TestResolveParamKey(t *testing.T) {
	if got := resolveParamKey("disable_quic", paths.DefaultMainSection); got != "hybrid-failover.settings.disable_quic" {
		t.Fatalf("got %q", got)
	}
	if got := resolveParamKey("urltest_interval", paths.DefaultMainSection); got != "hybrid-failover.glob.urltest_check_interval" {
		t.Fatalf("got %q", got)
	}
	if got := resolveParamKey("urltest_interval", "custom"); got != "hybrid-failover.custom.urltest_check_interval" {
		t.Fatalf("got %q", got)
	}
	if got := resolveParamKey("policy", "vpn1"); got != "hybrid-failover.vpn1.failover_policy" {
		t.Fatalf("got %q", got)
	}
}

func TestUCISectionKey(t *testing.T) {
	if got := uciSectionKey("", "home", "urltest_tolerance"); got != "hybrid-failover.home.urltest_tolerance" {
		t.Fatalf("got %q", got)
	}
	if got := uciSectionKey("custom-pkg", "sec", "failover_policy"); got != "custom-pkg.sec.failover_policy" {
		t.Fatalf("got %q", got)
	}
}
