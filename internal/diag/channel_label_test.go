package diag

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestChannelLabelPrefersChannelNames(t *testing.T) {
	link := "vless://uuid@1.2.3.4:443?security=reality#x"
	pkg, err := uci.Parse("config section 'main'\n\tlist channel_names '" + channels.ID(link) + "=Латвия, Рига'\n")
	if err != nil {
		t.Fatal(err)
	}
	names := channels.Names(pkg.Section("main"))
	if got := channelLabel(names, link, "main-1-out"); got != "Латвия, Рига (main-1-out)" {
		t.Fatalf("got %q", got)
	}
	if got := channelLabel(names, "vless://other@5.6.7.8:443", "main-2-out"); got != shortProxyLabel("vless://other@5.6.7.8:443")+" (main-2-out)" {
		t.Fatalf("unnamed: %q", got)
	}
}
