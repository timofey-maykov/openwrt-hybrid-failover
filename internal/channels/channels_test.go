package channels

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// A section switched from vpn to proxy outside LuCI keeps its channels in
// failover_proxy_links; the proxy mode must still find them.
func TestLinksProxyFallsBackToFailoverList(t *testing.T) {
	pkg, err := uci.Parse(`
config section 'glob'
	option connection_type 'proxy'
	option proxy_config_type 'urltest'
	list failover_proxy_links 'hy2://pass@203.0.113.10:443'
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := Links(pkg.Section("glob")); len(got) != 1 {
		t.Fatalf("links %v", got)
	}
}

func TestLinksProxyPrefersItsOwnList(t *testing.T) {
	pkg, err := uci.Parse(`
config section 'glob'
	option connection_type 'proxy'
	option proxy_config_type 'urltest'
	list urltest_proxy_links 'hy2://pass@203.0.113.10:443'
	list urltest_proxy_links 'hy2://pass@203.0.113.11:443'
	list failover_proxy_links 'hy2://pass@198.51.100.24:443'
`)
	if err != nil {
		t.Fatal(err)
	}
	if got := Links(pkg.Section("glob")); len(got) != 2 {
		t.Fatalf("links %v", got)
	}
}
