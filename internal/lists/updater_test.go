package lists

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
	u.RulesetDir = t.TempDir()
	u.UCIPath = uciPath
	u.HTTP = srv.Client()

	res, err := u.UpdateOnce()
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected changed=true for new telegram.lst")
	}
}
