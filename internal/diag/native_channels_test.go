package diag

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/failover"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestChannelSelectedPinnedVsURLTestWinner(t *testing.T) {
	group := singbox.URLTestTag("main")
	hy2 := "main-2-out"
	awg := "main-3-out"
	if !channelSelected(hy2, group, hy2, group) {
		t.Fatal("urltest winner selected when selector is the group")
	}
	if channelSelected(awg, group, hy2, group) {
		t.Fatal("other member selected")
	}
	if !channelSelected(awg, awg, hy2, group) {
		t.Fatal("pinned member not selected")
	}
	if channelSelected(hy2, awg, hy2, group) {
		t.Fatal("urltest winner stays highlighted while selector is pinned elsewhere")
	}
}

func TestActiveFromController(t *testing.T) {
	states := []failover.SectionRuntime{{
		Section: "glob",
		Active:  singbox.URLTestTag("glob"),
	}}
	if got := activeFromController(states, "glob"); got != singbox.URLTestTag("glob") {
		t.Fatalf("got %q", got)
	}
}

func TestEnrichNativeReportSetsActiveOutbound(t *testing.T) {
	pkg, err := uci.Parse(`
config section 'glob'
	option connection_type 'vpn'
	option failover_vpn_enabled '1'
	option interface 'awg0'
	list failover_proxy_links 'hy2://user:pass@1.2.3.4:443'
`)
	if err != nil {
		t.Fatal(err)
	}
	sec := pkg.Section("glob")
	states := []failover.SectionRuntime{{
		Section: "glob",
		Active:  singbox.URLTestTag("glob"),
		Mode:    "backup",
	}}
	r := EnrichNativeReport(Report{EngineMode: "native"}, "glob", sec, states)
	if r.ActiveOutbound != singbox.URLTestTag("glob") {
		t.Fatalf("active_outbound: %q", r.ActiveOutbound)
	}
	if r.Failover == nil || r.Failover.SelectorNow != singbox.URLTestTag("glob") {
		t.Fatalf("failover: %+v", r.Failover)
	}
	if len(r.Channels) < 3 {
		t.Fatalf("channels: %d", len(r.Channels))
	}
}
