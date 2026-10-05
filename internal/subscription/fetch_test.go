package subscription

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const subConf = "[Interface]\nPrivateKey = cHJpdmF0ZS1rZXktZm9yLXRlc3RzLW9ubHktMDAwMDA=\nAddress = 10.78.0.2/32\nJc = 4\nS1 = 67\nS2 = 66\n" +
	"[Peer]\nPublicKey = c2VydmVyLXB1YmxpYy1rZXktZm9yLXRlc3RzLW9ubHkwMDA=\nAllowedIPs = 0.0.0.0/0, ::/0\nEndpoint = 192.0.2.10:41491\n"

func serve(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchURLs_AmneziaWGEntries(t *testing.T) {
	entry := "amneziawg://" + base64.StdEncoding.EncodeToString([]byte(subConf)) + "#Latvia"
	// the whole subscription is base64 too, as panels send it
	url := serve(t, base64.StdEncoding.EncodeToString([]byte(entry+"\n"+entry+"\n")))

	links, err := NewFetcher().FetchURLs([]string{url})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1 (duplicates collapse): %v", len(links), links)
	}
	if !strings.HasPrefix(links[0], "awg2://192.0.2.10:41491?") {
		t.Fatalf("link was not converted to awg2://: %s", links[0])
	}
}

func TestFetchURLs_SkipsBrokenAmneziaWGEntry(t *testing.T) {
	url := serve(t, "amneziawg://!!!\nhy2://user:pass@192.0.2.20:443?sni=example.com\n")
	links, err := NewFetcher().FetchURLs([]string{url})
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || !strings.HasPrefix(links[0], "hy2://") {
		t.Fatalf("links = %v", links)
	}
}
