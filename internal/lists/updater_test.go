package lists

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

func TestFetchSubnetListDetectsNewEntries(t *testing.T) {
	remote := []byte("149.154.160.0/20\n91.108.4.0/22\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(remote)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "telegram.lst")
	if err := os.WriteFile(dest, []byte("149.154.160.0/20\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	u := NewUpdater(false)
	u.RulesetDir = dir
	u.HTTP = srv.Client()

	changed, err := u.fetchSubnetList(srv.URL, "telegram.lst")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changed=true when remote adds CIDR")
	}
}

func TestConfiguredServicesFromUCI(t *testing.T) {
	uciPath := filepath.Join(t.TempDir(), "hybrid-failover")
	body := "" +
		"config section 'main'\n" +
		"\toption connection_type 'vpn'\n" +
		"\tlist community_lists 'telegram'\n" +
		"\tlist community_lists 'meta'\n" +
		"\tlist community_lists 'russia_inside'\n"
	if err := os.WriteFile(uciPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	u := NewUpdater(false)
	u.UCIPath = uciPath
	got := u.configuredServices()
	if len(got) != 3 {
		t.Fatalf("unexpected services: %v", got)
	}
}

func TestUpdateOnceSkipsFailedDownloadWithCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusInternalServerError)
	}))
	defer srv.Close()

	uciPath := filepath.Join(t.TempDir(), "hybrid-failover")
	body := "" +
		"config section 'main'\n" +
		"\toption connection_type 'vpn'\n" +
		"\tlist community_lists 'telegram'\n"
	if err := os.WriteFile(uciPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, "telegram.lst")
	if err := os.WriteFile(dest, []byte("149.154.160.0/20\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := singbox.WriteDomainRuleset(
		filepath.Join(dir, singbox.RulesetTag("main", "telegram", "community")+".json"),
		[]string{"t.me"},
	); err != nil {
		t.Fatal(err)
	}

	prev := subnetListURL
	subnetListURL = func(service string) (string, bool) {
		if service == "telegram" {
			return srv.URL, true
		}
		return prev(service)
	}
	t.Cleanup(func() { subnetListURL = prev })

	u := NewFromUCI(uciPath)
	u.RulesetDir = dir
	u.HTTP = offlineHTTPClient()

	res, err := u.UpdateOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("expected changed=false when remote fails with valid cache")
	}
}

func TestUpdateOnceSkipsServicesWithoutSubnetList(t *testing.T) {
	remote := []byte("149.154.160.0/20\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(remote)
	}))
	defer srv.Close()

	uciPath := filepath.Join(t.TempDir(), "hybrid-failover")
	body := "" +
		"config section 'main'\n" +
		"\toption connection_type 'vpn'\n" +
		"\tlist community_lists 'russia_inside'\n" +
		"\tlist community_lists 'telegram'\n"
	if err := os.WriteFile(uciPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	u := NewUpdater(false)
	dir := t.TempDir()
	u.RulesetDir = dir
	u.UCIPath = uciPath
	u.HTTP = srv.Client()
	if err := singbox.WriteDomainRuleset(
		filepath.Join(dir, singbox.RulesetTag("main", "russia_inside", "community")+".json"),
		[]string{"example.ru"},
	); err != nil {
		t.Fatal(err)
	}

	res, err := u.UpdateOnce()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true for new telegram.lst")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func offlineHTTPClient() *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("offline")
		}),
	}
}
