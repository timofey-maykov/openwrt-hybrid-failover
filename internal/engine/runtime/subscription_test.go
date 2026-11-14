package runtime

import (
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"reflect"
	"testing"
)

func TestSubscriptionChannelsExcludeWANAndSelectors(t *testing.T) {
	p := &plan.Plan{Outbounds: []plan.OutboundPlan{
		{Tag: "direct", Kind: plan.OutboundDirect},
		{Tag: "selector", Kind: plan.OutboundSelector, Members: []string{"vpn", "direct"}},
		{Tag: "vpn", Kind: plan.OutboundDirectBind, BindIface: "awg0"},
		{Tag: "unbound-json-direct", Kind: plan.OutboundDirectBind},
		{Tag: "json-socks", Kind: plan.OutboundSocks},
		{Tag: "hy2", Kind: plan.OutboundHysteria2},
		{Tag: "awg2", Kind: plan.OutboundAWG2Bind, BindIface: "awg1"},
	}}
	var tags []string
	for _, ch := range subscriptionChannels(p) {
		tags = append(tags, ch.Tag)
	}
	if !reflect.DeepEqual(tags, []string{"vpn", "json-socks", "hy2", "awg2"}) {
		t.Fatal(tags)
	}
}
