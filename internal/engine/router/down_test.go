package router

import (
	"context"
	"net"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func downPlan(directWhenDown bool) *plan.Plan {
	return &plan.Plan{
		DNS: plan.DNSPlan{FakeIPRange: plan.FakeIPRange},
		Sections: []plan.SectionPlan{{
			Name: "main", ConnectionType: "proxy", ListBased: true,
			SelectorTag: "main-out", DirectWhenDown: directWhenDown,
		}},
		Outbounds: []plan.OutboundPlan{
			{Tag: "direct-out", Kind: plan.OutboundDirect},
			{Tag: "main-1-out", Kind: plan.OutboundDirect},
			{Tag: "main-2-out", Kind: plan.OutboundDirect},
			{Tag: "main-out", Kind: plan.OutboundSelector, Members: []string{"main-1-out", "main-2-out"}, Default: "main-1-out"},
			{Tag: "main-route-lr_work-out", Kind: plan.OutboundFallback, Members: []string{"main-2-out"}},
		},
		Channels: []plan.ChannelPlan{
			{Section: "main", ID: "a", Tag: "main-1-out"},
			{Section: "main", ID: "b", Tag: "main-2-out"},
		},
		Bindings: []plan.BindingPlan{{Name: "lr_work", Section: "main", Channel: "b", OnDown: "block", OutboundTag: "main-route-lr_work-out"}},
		Routes: []plan.RouteRule{
			{Action: "route", OutboundTag: "main-route-lr_work-out", Binding: "lr_work", DomainSuffix: []string{"work.example"}},
			{Action: "route", OutboundTag: "main-out", Section: "main", DomainSuffix: []string{"youtube.com"}},
		},
	}
}

func downControl(p *plan.Plan, alive ...string) *control.Control {
	ctrl := control.New()
	ctrl.BindPlan(p)
	up := map[string]bool{}
	for _, t := range alive {
		up[t] = true
	}
	for _, ch := range p.Channels {
		if up[ch.Tag] {
			ctrl.SetDelay(ch.Tag, plan.DelaySample{Tag: ch.Tag, OK: true, Delay: 80})
		} else {
			ctrl.SetDelay(ch.Tag, plan.DelaySample{Tag: ch.Tag, Error: "timeout"})
		}
	}
	return ctrl
}

func TestRouteAllChannelsDownGoesDirect(t *testing.T) {
	p := downPlan(true)
	r, err := New(p, nil, downControl(p))
	if err != nil {
		t.Fatal(err)
	}
	tag, err := r.Route(plan.ConnMeta{Domain: "www.youtube.com", DstPort: 443})
	if err != nil || tag != plan.DirectTag {
		t.Fatalf("all channels down: got %q %v, want direct", tag, err)
	}
	// A list bound with on_down block stays out of the direct path.
	rt, err := r.route(plan.ConnMeta{Domain: "jira.work.example", DstPort: 443})
	if err != nil || rt.tag == plan.DirectTag || rt.section != "" {
		t.Fatalf("on_down block must not go direct: %+v %v", rt, err)
	}
}

func TestRouteOneChannelAliveKeepsTunnel(t *testing.T) {
	p := downPlan(true)
	r, err := New(p, nil, downControl(p, "main-2-out"))
	if err != nil {
		t.Fatal(err)
	}
	tag, err := r.Route(plan.ConnMeta{Domain: "www.youtube.com", DstPort: 443})
	if err != nil || tag == plan.DirectTag {
		t.Fatalf("a live channel must keep the tunnel, got %q %v", tag, err)
	}
}

func TestRouteDirectWhenDownOff(t *testing.T) {
	p := downPlan(false)
	r, err := New(p, nil, downControl(p))
	if err != nil {
		t.Fatal(err)
	}
	rt, err := r.route(plan.ConnMeta{Domain: "www.youtube.com", DstPort: 443})
	if err != nil || rt.tag == plan.DirectTag || rt.section != "" {
		t.Fatalf("option off: traffic must not go direct, got %+v %v", rt, err)
	}
}

func TestRouteUnprobedChannelsCountAsAlive(t *testing.T) {
	p := downPlan(true)
	ctrl := control.New()
	ctrl.BindPlan(p)
	r, err := New(p, nil, ctrl)
	if err != nil {
		t.Fatal(err)
	}
	tag, _ := r.Route(plan.ConnMeta{Domain: "www.youtube.com", DstPort: 443})
	if tag == plan.DirectTag {
		t.Fatal("before the first probe the tunnel must be tried")
	}
}

// A tunnel that died between two probes: the dial through it fails, the
// connection must still get through directly.
func TestDialFallsBackDirectWhenTunnelDialFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	p := &plan.Plan{
		DNS:      plan.DNSPlan{FakeIPRange: plan.FakeIPRange},
		Sections: []plan.SectionPlan{{Name: "main", ConnectionType: "proxy", ListBased: true, DirectWhenDown: true}},
		Outbounds: []plan.OutboundPlan{
			{Tag: "direct-out", Kind: plan.OutboundDirect},
			// a SOCKS server nobody listens on: every dial through it fails
			{Tag: "main-out", Kind: plan.OutboundSocks, ProxyURI: "socks5://127.0.0.1:1"},
		},
		Channels: []plan.ChannelPlan{{Section: "main", ID: "a", Tag: "main-out"}},
		Routes:   []plan.RouteRule{{Action: "route", OutboundTag: "main-out", Section: "main", IPCIDR: []string{"127.0.0.1/32"}}},
	}
	reg, err := outbound.NewRegistry(context.Background(), p.Outbounds)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Stop()
	ctrl := control.New()
	ctrl.BindPlan(p)
	r, err := New(p, reg, ctrl)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.Dial(plan.ConnMeta{DstIP: "127.0.0.1", DstPort: port, Network: "tcp"})
	if err != nil {
		t.Fatalf("must fall back direct: %v", err)
	}
	c.Close()

	p.Sections[0].DirectWhenDown = false
	r, _ = New(p, reg, ctrl)
	if c, err := r.Dial(plan.ConnMeta{DstIP: "127.0.0.1", DstPort: port, Network: "tcp"}); err == nil {
		c.Close()
		t.Fatal("option off: a failed tunnel dial must stay failed")
	}
}
