package plan_test

import (
	"fmt"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
	"testing"
)

func TestUserListDirectSubnetPrecedesSectionPool(t *testing.T) {
	pkg, err := uci.Parse(fmt.Sprintf(bindingsConf, "direct"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := plan.CompilePlan(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Routes) == 0 || p.Routes[0].Binding != "r_work" || p.Routes[0].OutboundTag != plan.DirectTag || p.Routes[0].Reject {
		t.Fatalf("direct exception missing or ordered incorrectly: %+v", p.Routes)
	}
	var subnets bool
	for _, rs := range p.RuleSets {
		for _, cidr := range rs.Subnets {
			subnets = subnets || cidr == "10.20.0.0/16"
		}
	}
	if !subnets {
		t.Fatal("direct subnet exception missing")
	}
}
