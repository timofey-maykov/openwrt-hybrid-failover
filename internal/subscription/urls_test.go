package subscription

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestFetchIgnoresEmptyUCIRows(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte("hy2://password@192.0.2.20:443#Working"))
	}))
	defer server.Close()
	pkg, err := uci.Parse("config settings 'settings'\n list subscription_urls ''\n list subscription_urls ' " + server.URL + " '\n list subscription_urls ''\n list subscription_urls '" + server.URL + "'\n")
	if err != nil {
		t.Fatal(err)
	}
	f := NewFetcher()
	f.FallbackClients = func(context.Context) []*http.Client { t.Fatal("unnecessary fallback"); return nil }
	res, err := f.Fetch(pkg.Section("settings").GetList("subscription_urls"))
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(res.Sources) != 1 || len(res.Links) != 1 {
		t.Fatalf("requests=%d sources=%d links=%d", requests, len(res.Sources), len(res.Links))
	}
}

func TestInvalidSubscriptionURLHasUsefulError(t *testing.T) {
	f := NewFetcher()
	f.FallbackClients = func(context.Context) []*http.Client { t.Fatal("bad URL must not discover tunnels"); return nil }
	for _, raw := range []string{"", "panel.example/private-token", "ftp://panel.example/private-token", "https:///private-token"} {
		_, err := f.FetchOne(raw)
		if err == nil || !strings.Contains(err.Error(), "complete http:// or https://") || strings.Contains(err.Error(), "private-token") {
			t.Fatalf("error=%v", err)
		}
	}
}

func TestProviderCompatibilityNoticeIsNotImported(t *testing.T) {
	notice := "vless://00000000-0000-0000-0000-000000000000@192.0.2.1:443?security=none#" + url.PathEscape("🔴 Приложение не поддерживается!")
	for _, body := range []string{notice, base64.StdEncoding.EncodeToString([]byte(notice))} {
		f := NewFetcher()
		f.FallbackClients = nil
		res, err := f.Fetch([]string{serve(t, body)})
		if err == nil || !strings.Contains(err.Error(), "Приложение не поддерживается") {
			t.Fatalf("error=%v", err)
		}
		if len(res.Links) != 0 {
			t.Fatal("imported a provider notice")
		}
	}
	// A real server must not be discarded merely because a panel also supplied
	// a placeholder notice in the same subscription.
	if got := providerNotice([]string{notice, "hy2://password@192.0.2.20:443"}); got != "" {
		t.Fatalf("mixed subscription rejected: %q", got)
	}
}

func TestInvalidSubscriptionSourceNeverStoresUnmaskedToken(t *testing.T) {
	f := NewFetcher()
	result, err := f.Fetch([]string{"panel.example/private-token"})
	if err == nil || len(result.Sources) != 1 || strings.Contains(result.Sources[0].URL, "private-token") {
		t.Fatal("invalid subscription token leaked into status")
	}
}
