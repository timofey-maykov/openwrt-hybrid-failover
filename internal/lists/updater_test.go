package lists

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTextBodyIfChanged(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "telegram.txt")
	body := []byte("telegram.org\nt.me\n")
	changed, err := writeTextBodyIfChanged(dest, body)
	if err != nil || !changed {
		t.Fatalf("expected first write changed=true, err=%v changed=%v", err, changed)
	}
	changed, err = writeTextBodyIfChanged(dest, body)
	if err != nil || changed {
		t.Fatalf("expected unchanged write, err=%v changed=%v", err, changed)
	}
}

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
		"\tlist community_lists 'meta'\n"
	if err := os.WriteFile(uciPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	u := NewUpdater(false)
	u.UCIPath = uciPath
	got := u.configuredServices()
	if len(got) != 2 || got[0] != "telegram" || got[1] != "meta" {
		t.Fatalf("unexpected services: %v", got)
	}
}
