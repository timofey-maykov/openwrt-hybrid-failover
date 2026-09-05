package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestRouteListBasedUnmatchedGoesDirect(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.json")
	body := `{"version":3,"rules":[{"domain_suffix":["youtube.com","googlevideo.com"]}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{
		DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange},
		Sections: []plan.SectionPlan{{
			Name:           "main",
			ConnectionType: "vpn",
			ListBased:      true,
			SelectorTag:    "main-out",
		}},
		Outbounds: []plan.OutboundPlan{
			{Tag: "direct-out", Kind: plan.OutboundDirect},
			{Tag: "main-out", Kind: plan.OutboundSelector},
		},
		RuleSets: []plan.RuleSet{{
			Tag:  "main-russia_inside-community",
			Kind: "domains",
			Path: path,
		}},
		Routes: []plan.RouteRule{{
			Action:      "route",
			OutboundTag: "main-out",
			Section:     "main",
			RuleSetTags: []string{"main-russia_inside-community"},
		}},
	}
	r, err := New(p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	tag, err := r.Route(plan.ConnMeta{Domain: "music.yandex.ru", DstIP: "213.180.204.186", DstPort: 443})
	if err != nil {
		t.Fatal(err)
	}
	if tag != plan.DirectTag {
		t.Fatalf("yandex must stay direct, got %q", tag)
	}

	tag, err = r.Route(plan.ConnMeta{Domain: "www.youtube.com", DstPort: 443})
	if err != nil {
		t.Fatal(err)
	}
	if tag != "main-out" {
		t.Fatalf("youtube must use section outbound, got %q", tag)
	}

	if r.ShouldFakeIP("music.yandex.ru") {
		t.Fatal("yandex must not FakeIP")
	}
	if !r.ShouldFakeIP("www.youtube.com") {
		t.Fatal("youtube must FakeIP")
	}
}

func TestRouteFullRouteSourceOverridesDirect(t *testing.T) {
	p := &plan.Plan{
		DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange},
		Sections: []plan.SectionPlan{{
			Name:           "main",
			ConnectionType: "vpn",
			ListBased:      true,
		}},
		Outbounds: []plan.OutboundPlan{
			{Tag: "direct-out", Kind: plan.OutboundDirect},
			{Tag: "main-out", Kind: plan.OutboundSelector},
		},
		Routes: []plan.RouteRule{{
			Action:       "route",
			OutboundTag:  "main-out",
			Section:      "main",
			SourceIPCIDR: []string{"192.168.11.50"},
		}},
	}
	r, err := New(p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := r.Route(plan.ConnMeta{
		SrcIP:  "192.168.11.50",
		DstIP:  "213.180.204.186",
		Domain: "music.yandex.ru",
		DstPort: 443,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tag != "main-out" {
		t.Fatalf("full_route client must use section, got %q", tag)
	}
}

func TestRouteCatchAllWithoutLists(t *testing.T) {
	p := &plan.Plan{
		DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange},
		Sections: []plan.SectionPlan{{
			Name:           "main",
			ConnectionType: "vpn",
			ListBased:      false,
		}},
		Outbounds: []plan.OutboundPlan{
			{Tag: "direct-out", Kind: plan.OutboundDirect},
			{Tag: "main-out", Kind: plan.OutboundSelector},
		},
		Routes: []plan.RouteRule{{
			Action:      "route",
			OutboundTag: "main-out",
			Section:     "main",
		}},
	}
	r, err := New(p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := r.Route(plan.ConnMeta{Domain: "music.yandex.ru", DstPort: 443})
	if err != nil {
		t.Fatal(err)
	}
	if tag != "main-out" {
		t.Fatalf("section without lists catch-all to proxy, got %q", tag)
	}
}
