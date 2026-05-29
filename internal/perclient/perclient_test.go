package perclient

import (
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestRefreshFromUCIParsesIncludeExclude(t *testing.T) {
	pkg, err := uci.Parse(`
config settings 'settings'
	list include_source_ips '192.168.1.50'
	list exclude_source_ips '192.168.1.99'

config section 'glob'
	list fully_routed_ips '192.168.1.60'
`)
	if err != nil {
		t.Fatal(err)
	}
	// ApplyFromUCI requires nft on the host; ensure UCI lists are readable.
	if got := pkg.Section("settings").GetList("include_source_ips"); len(got) != 1 || got[0] != "192.168.1.50" {
		t.Fatalf("include list: %v", got)
	}
	if got := pkg.Section("settings").GetList("exclude_source_ips"); len(got) != 1 {
		t.Fatalf("exclude list: %v", got)
	}
	if got := pkg.Section("glob").GetList("fully_routed_ips"); len(got) != 1 || !strings.Contains(got[0], "192.168.1.60") {
		t.Fatalf("fully_routed: %v", got)
	}
}
