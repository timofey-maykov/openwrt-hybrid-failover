package subnets_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
)

func TestCIDRSetsEqual(t *testing.T) {
	a := []string{"149.154.160.0/20", "91.108.4.0/22"}
	b := []string{"91.108.4.0/22", "149.154.160.0/20"}
	if !subnets.CIDRSetsEqual(a, b) {
		t.Fatal("expected equal sets")
	}
	if subnets.CIDRSetsEqual(a, append(b, "5.28.192.0/18")) {
		t.Fatal("expected different sets")
	}
}

func TestRefreshFileIfChangedSkipsIdentical(t *testing.T) {
	body := []byte("149.154.160.0/20\n91.108.4.0/22\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "telegram.lst")
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := subnets.RefreshFileIfChanged(srv.URL, dest, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no write for identical remote")
	}
}

func TestRefreshFileIfChangedUpdatesNewEntries(t *testing.T) {
	local := []byte("149.154.160.0/20\n")
	remote := []byte("149.154.160.0/20\n91.108.4.0/22\n")
	var served []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(served)
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "telegram.lst")
	if err := os.WriteFile(dest, local, 0o644); err != nil {
		t.Fatal(err)
	}

	served = local
	changed, err := subnets.RefreshFileIfChanged(srv.URL, dest, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected unchanged for same entries")
	}

	served = remote
	changed, err = subnets.RefreshFileIfChanged(srv.URL, dest, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected update when remote has new CIDR")
	}
	got, err := subnets.ParseFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !subnets.CIDRSetsEqual(got, []string{"149.154.160.0/20", "91.108.4.0/22"}) {
		t.Fatalf("unexpected dest content: %v", got)
	}
}
