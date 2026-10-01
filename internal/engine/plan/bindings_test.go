package plan_test

import (
	"fmt"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const bindingsConf = `
config settings 'settings'
	option enabled '1'

config section 'main'
	option connection_type 'proxy'
	option proxy_config_type 'urltest'
	list urltest_proxy_links 'vless://11111111-1111-1111-1111-111111111111@a.example.com:443?encryption=none&security=reality&sni=a.example.com&fp=chrome&pbk=VDx8FnyKEJntMxyrVqRXJqfdhnnz9tNTsQr064RBTWU&sid=abcd&type=tcp'
	list urltest_proxy_links 'vless://22222222-2222-2222-2222-222222222222@b.example.com:443?encryption=none&security=reality&sni=b.example.com&fp=chrome&pbk=VDx8FnyKEJntMxyrVqRXJqfdhnnz9tNTsQr064RBTWU&sid=abcd&type=tcp'

config user_list 'work'
	option section 'main'
	option subnets_text '10.20.0.0/16'

config user_list 'games'
	option section 'main'
	option subnets_text '10.30.0.0/16'

config user_list 'lan2'
	option section 'main'
	option subnets_text '10.40.0.0/16'

config list_route 'r_work'
	option section 'main'
	list lists 'user:work'
	option channel '%s'
	option on_down 'direct'

config list_route 'r_games'
	option section 'main'
	list lists 'user:games'
	option channel 'balance'
`

func TestCompileListBindings(t *testing.T) {
	linkB := "vless://22222222-2222-2222-2222-222222222222@b.example.com:443?encryption=none&security=reality&sni=b.example.com&fp=chrome&pbk=VDx8FnyKEJntMxyrVqRXJqfdhnnz9tNTsQr064RBTWU&sid=abcd&type=tcp"
	idB := channels.ID(linkB)
	pkg, err := uci.Parse(sprintf(bindingsConf, idB))
	if err != nil {
		t.Fatal(err)
	}
	p, err := plan.CompilePlan(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Channels) != 2 || p.Channels[1].ID != idB || p.Channels[1].Tag != plan.OutboundTag("main-2") {
		t.Fatalf("channels: %+v", p.Channels)
	}
	if len(p.Bindings) != 2 {
		t.Fatalf("bindings: %+v", p.Bindings)
	}
	work := p.Bindings[0]
	if work.ChannelTag != plan.OutboundTag("main-2") || work.OutboundTag != plan.BindingTag("main", "r_work") {
		t.Fatalf("work binding: %+v", work)
	}
	var fb, bal *plan.OutboundPlan
	for i := range p.Outbounds {
		switch p.Outbounds[i].Tag {
		case plan.BindingTag("main", "r_work"):
			fb = &p.Outbounds[i]
		case plan.BindingTag("main", "r_games"):
			bal = &p.Outbounds[i]
		}
	}
	if fb == nil || fb.Kind != plan.OutboundFallback || len(fb.Members) != 2 || fb.Members[1] != plan.DirectTag {
		t.Fatalf("fallback outbound: %+v", fb)
	}
	if bal == nil || bal.Kind != plan.OutboundBalance || len(bal.Members) != 2 {
		t.Fatalf("balance outbound: %+v", bal)
	}
	// Binding rules come first and carry no Section; the unbound list stays
	// in the section rule.
	if len(p.Routes) != 3 {
		t.Fatalf("routes: %+v", p.Routes)
	}
	if p.Routes[0].Binding != "r_work" || p.Routes[0].Section != "" || p.Routes[1].Binding != "r_games" {
		t.Fatalf("binding rules: %+v", p.Routes[:2])
	}
	base := p.Routes[2]
	if base.Section != "main" || len(base.RuleSetTags) != 1 || base.RuleSetTags[0] != plan.RulesetTag("main", "ul-lan2", "subnets") {
		t.Fatalf("section rule: %+v", base)
	}
	if !p.Sections[0].ListBased {
		t.Fatal("a section with user lists must route by lists")
	}
}

func TestCompileListBindingMissingChannel(t *testing.T) {
	pkg, err := uci.Parse(sprintf(bindingsConf, "deadbeef"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := plan.CompilePlan(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Bindings[0].Missing {
		t.Fatalf("binding to a removed channel must be marked missing: %+v", p.Bindings[0])
	}
	// The list falls back to the section rule.
	base := p.Routes[len(p.Routes)-1]
	if len(base.RuleSetTags) != 2 {
		t.Fatalf("section rule must keep work and lan2: %+v", base)
	}
}

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }
