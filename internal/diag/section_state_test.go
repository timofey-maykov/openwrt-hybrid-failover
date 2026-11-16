package diag

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestDisabledSectionDoesNotExposeOrProbeStaleChannels(t *testing.T) {
	sec := &uci.Section{Options: map[string]string{"enabled": "0", "connection_type": "proxy", "proxy_config_type": "urltest"}, Lists: map[string][]string{"urltest_proxy_links": {"hy2://secret@192.0.2.1:443"}}}
	stale := Report{EngineMode: "native", EngineRunning: true, ActiveOutbound: "glob-1-out", Failover: &FailoverInfo{Section: "glob", SelectorNow: "glob-1-out"}, Channels: []ChannelStatus{{Name: "glob-1-out", Probed: true, Detail: "outbound not found"}}, Controller: []ControllerSection{{Section: "glob", Active: "glob-1-out"}, {Section: "other", Active: "other-out"}}}
	for name, fn := range map[string]func(Report) Report{
		"native status": func(r Report) Report { return EnrichNativeReport(r, "glob", sec, nil) },
		"native health": func(r Report) Report { return ProbeNativeChannels(r, "glob", sec) },
		"legacy status": func(r Report) Report {
			r.EngineMode = "singbox"
			r.ClashOK = true
			return EnrichReport(r, "http://127.0.0.1:1", "glob", sec)
		},
		"legacy health": func(r Report) Report {
			r.EngineMode = "singbox"
			r.ClashOK = true
			return ProbeChannels(r, "http://127.0.0.1:1", "glob", sec)
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := fn(stale)
			if !r.SectionDisabled || r.ActiveOutbound != "" || len(r.Channels) != 0 || r.Failover.SelectorNow != "" {
				t.Fatalf("stale disabled data: %+v", r)
			}
			if len(r.Controller) != 1 || r.Controller[0].Section != "other" {
				t.Fatalf("other section affected: %+v", r.Controller)
			}
		})
	}
	sec.Options["enabled"] = "1"
	if WithSectionState(stale, "glob", sec).SectionDisabled {
		t.Fatal("enabled section marked disabled")
	}
	delete(sec.Options, "enabled")
	if WithSectionState(stale, "glob", sec).SectionDisabled {
		t.Fatal("default enabled section marked disabled")
	}
}
