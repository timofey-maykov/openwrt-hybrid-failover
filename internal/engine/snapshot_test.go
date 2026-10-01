package engine

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

// A proxy bound to the WAN must not report the WAN interface counters as its
// own traffic: two proxies on one WAN showed the same bytes.
func TestChannelRuntimeIfaceOnlyForTunnels(t *testing.T) {
	p := &plan.Plan{
		Outbounds: []plan.OutboundPlan{
			{Tag: "main-1-out", Kind: plan.OutboundHysteria2, BindIface: "pppoe-wan"},
			{Tag: "main-2-out", Kind: plan.OutboundAWG2Bind, BindIface: "pawg1"},
		},
		Channels: []plan.ChannelPlan{
			{Section: "main", ID: "a", Tag: "main-1-out"},
			{Section: "main", ID: "b", Tag: "main-2-out"},
		},
	}
	got := channelRuntime(p, map[string]outbound.TrafficStat{"main-1-out": {Rx: 100, Tx: 10}})
	if got[0].Iface != "" || got[0].Rx != 100 || got[0].Tx != 10 {
		t.Fatalf("proxy channel: %+v", got[0])
	}
	if got[1].Iface != "pawg1" {
		t.Fatalf("awg channel: %+v", got[1])
	}
}
