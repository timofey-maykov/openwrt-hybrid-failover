package router

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestRouteUDPRoutedOnlyUDP(t *testing.T) {
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
			SourceIPCIDR: []string{"192.168.11.214"},
			Network:      "udp",
		}},
	}
	r, err := New(p, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	game := plan.ConnMeta{SrcIP: "192.168.11.214", DstIP: "18.156.219.186", DstPort: 9034, Network: "udp"}
	if tag, err := r.Route(game); err != nil || tag != "main-out" {
		t.Fatalf("console UDP must use the section, got %q, %v", tag, err)
	}
	web := plan.ConnMeta{SrcIP: "192.168.11.214", DstIP: "18.156.219.186", DstPort: 443, Network: "tcp"}
	if tag, err := r.Route(web); err != nil || tag != plan.DirectTag {
		t.Fatalf("console TCP must stay direct, got %q, %v", tag, err)
	}
	other := plan.ConnMeta{SrcIP: "192.168.11.50", DstIP: "18.156.219.186", DstPort: 9034, Network: "udp"}
	if tag, err := r.Route(other); err != nil || tag != plan.DirectTag {
		t.Fatalf("other clients must stay direct, got %q, %v", tag, err)
	}
}
