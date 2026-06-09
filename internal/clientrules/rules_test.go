package clientrules

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestListRulesFromClientRule(t *testing.T) {
	pkg, err := uci.Parse(`
config client_rule 'phone'
	option ip '192.168.1.50'
	option mode 'include'
config client_rule 'full'
	option ip '192.168.1.60'
	option mode 'full_route'
	option section 'glob'
`)
	if err != nil {
		t.Fatal(err)
	}
	rules := ListRules(pkg)
	if len(rules) != 2 {
		t.Fatalf("rules: %+v", rules)
	}
	byName := make(map[string]Rule, len(rules))
	for _, r := range rules {
		byName[r.Name] = r
	}
	if byName["phone"].Mode != ModeInclude {
		t.Fatalf("phone mode: %s", byName["phone"].Mode)
	}
	if byName["full"].Mode != ModeFullRoute {
		t.Fatalf("full mode: %s", byName["full"].Mode)
	}
}

func TestLegacyRulesFallback(t *testing.T) {
	pkg, err := uci.Parse(`
config settings 'settings'
	list include_source_ips '192.168.1.50'
	list exclude_source_ips '192.168.1.99'
	list routing_excluded_ips '10.0.0.0/8'
config section 'glob'
	list fully_routed_ips '192.168.1.60'
`)
	if err != nil {
		t.Fatal(err)
	}
	rules := ListRules(pkg)
	if len(rules) != 4 {
		t.Fatalf("rules: %+v", rules)
	}
	if got := IncludeIPs(rules); len(got) != 2 {
		t.Fatalf("include: %v", got)
	}
	if got := ExcludeIPs(rules); len(got) != 1 {
		t.Fatalf("exclude: %v", got)
	}
}
