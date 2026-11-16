package subscription

import (
	"net/url"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

func TestPCSAndFragmentSurviveXrayImport(t *testing.T) {
	pin := strings.Repeat("AB", 32)
	body := `{"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"192.0.2.1","port":52006,"users":[{"id":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"}]}]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"serverName":"","fingerprint":"firefox","pinnedPeerCertSha256":"` + pin + `"},"finalmask":{"tcp":[{"type":"fragment","settings":{"packets":"tlshello","length":"10-20","interval":"10-20"}}]}}}]}`
	entries, err := parseSubscription([]byte(body))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
	u, _ := url.Parse(entries[0])
	q := u.Query()
	if q.Get("pcs") != pin || q.Get("sni") != "" || q.Get("fp") != "firefox" || !strings.Contains(q.Get("fm"), "tlshello") {
		t.Fatal("certificate pin or fragment lost during import")
	}
	parsed, err := uri.ParseProxy(entries[0], "test", false)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Fields["tls"].(map[string]any)["pinned_peer_cert_sha256"] != pin || parsed.Fields["finalmask"] != q.Get("fm") {
		t.Fatal("native parser lost imported settings")
	}
}
