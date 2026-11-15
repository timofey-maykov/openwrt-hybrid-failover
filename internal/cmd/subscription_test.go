package cmd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subscription"
)

func TestSubscriptionRefreshAllExcludedKeepsChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hy2://password@192.0.2.1:443#Test%20Server\n"))
	}))
	defer server.Close()
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	before := paths.SubscriptionStatus
	paths.SubscriptionStatus = filepath.Join(dir, "status.json")
	defer func() { paths.SubscriptionStatus = before }()
	conf := "config settings 'settings'\n list subscription_urls '" + server.URL + "'\n list subscription_exclude_keywords 'test'\nconfig section 'glob'\n option connection_type 'proxy'\n option proxy_config_type 'urltest'\n list urltest_proxy_links 'hy2://manual@192.0.2.2:443'\n"
	if err := os.WriteFile(config, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runSubscriptionRefresh([]string{"--uci", config}); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	status, ok := subscription.ReadStatus()
	if !ok || status.OK || status.Excluded != 1 || !strings.Contains(status.Error, "all subscription servers") {
		t.Fatalf("wrong status: %+v", status)
	}
	actual, _ := os.ReadFile(config)
	if string(actual) != conf {
		t.Fatal("existing channels changed when everything was excluded")
	}
	_ = os.Remove(paths.SubscriptionStatus)
	if code := runSubscriptionRefresh([]string{"--uci", config, "--dry-run"}); code != 1 {
		t.Fatalf("dry-run exit=%d", code)
	}
	if _, ok := subscription.ReadStatus(); ok {
		t.Fatal("dry-run wrote status")
	}
}

func TestSubscriptionRefreshExplainsInactiveVPNBeforeDownloading(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	before := paths.SubscriptionStatus
	paths.SubscriptionStatus = filepath.Join(dir, "status.json")
	defer func() { paths.SubscriptionStatus = before }()
	conf := "config settings 'settings'\n list subscription_urls 'https://127.0.0.1:1/private-token'\nconfig section 'glob'\n option connection_type 'vpn'\n option interface 'vpn0'\n option failover_vpn_enabled '0'\n"
	if err := os.WriteFile(config, []byte(conf), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runSubscriptionRefresh([]string{"--uci", config}); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	status, ok := subscription.ReadStatus()
	if !ok || status.OK || !strings.Contains(status.Error, "enable backup channels") || strings.Contains(status.Error, "private-token") {
		t.Fatalf("wrong status: %+v", status)
	}
	actual, _ := os.ReadFile(config)
	if string(actual) != conf {
		t.Fatal("VPN configuration changed")
	}
}
