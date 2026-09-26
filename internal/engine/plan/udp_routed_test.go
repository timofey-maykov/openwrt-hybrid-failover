package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// udp_routed_ips must turn into a UDP-only source rule for its section, ahead
// of the list rules; without it the router sent that UDP direct.
func TestCompileUDPRoutedRoutes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "testdata", "proxy-urltest.conf"))
	if err != nil {
		t.Skip("example not found:", err)
	}
	pkg, err := uci.Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	var section string
	for _, name := range pkg.SectionNames("section") {
		if s := pkg.Section(name); s != nil && (s.Get("connection_type", "") == "proxy" || s.Get("connection_type", "") == "vpn") {
			section = name
			break
		}
	}
	if section == "" {
		t.Skip("no proxy/vpn section in example")
	}
	header := "config section '" + section + "'\n"
	if !strings.Contains(string(raw), header) {
		t.Skip("unexpected example layout")
	}
	pkg, err = uci.Parse(strings.Replace(string(raw), header, header+"\tlist udp_routed_ips '192.168.11.214'\n", 1))
	if err != nil {
		t.Fatal(err)
	}

	p, err := plan.CompilePlan(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Routes) == 0 {
		t.Fatal("no routes")
	}
	first := p.Routes[0]
	if first.Network != "udp" || first.Section != section || len(first.SourceIPCIDR) != 1 || first.SourceIPCIDR[0] != "192.168.11.214" {
		t.Fatalf("first route must be the udp_routed rule, got %+v", first)
	}
	if first.OutboundTag != plan.OutboundTag(section) {
		t.Fatalf("outbound %q, want %q", first.OutboundTag, plan.OutboundTag(section))
	}
}
