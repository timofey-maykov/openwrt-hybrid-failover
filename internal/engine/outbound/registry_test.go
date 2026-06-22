package outbound

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

func TestExtractHostPort(t *testing.T) {
	if got := extractHostPort("https://www.gstatic.com/generate_204"); got != "www.gstatic.com:443" {
		t.Fatalf("generate_204: got %q", got)
	}
	if got := extractHostPort("http://1.1.1.1"); got != "1.1.1.1:80" {
		t.Fatalf("http host: got %q", got)
	}
}

func TestPickURLTestMemberTolerance(t *testing.T) {
	p := plan.OutboundPlan{
		Members: []string{"a", "b", "c"},
		URLTest: &plan.URLTestPlan{Tolerance: 50},
	}
	delays := map[string]int{"a": 200, "b": 180, "c": 500}
	if got := pickURLTestMember(p, delays, "a"); got != "a" {
		t.Fatalf("within tolerance keep current: got %q", got)
	}
	delays = map[string]int{"a": 300, "b": 100, "c": 500}
	if got := pickURLTestMember(p, delays, "a"); got != "b" {
		t.Fatalf("switch to faster: got %q want b", got)
	}
}
