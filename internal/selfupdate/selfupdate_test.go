package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, installed string
		want              bool
	}{
		{"1.7.46", "1.7.45", true},
		{"v1.8.0", "1.7.45", true},
		{"1.7.45", "1.7.45", false},
		{"1.7.44", "1.7.45", false},
		{"1.10.0", "1.9.9", true},
		{"1.7.46", "1.7.45-pprof", true},
		{"1.7.46", "dev", false},
		{"garbage", "1.7.45", false},
	}
	for _, c := range cases {
		if got := Newer(c.latest, c.installed); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.latest, c.installed, got, c.want)
		}
	}
}

func loadManifest(t *testing.T) Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "manifest-1.7.45.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestArchCandidatesBE7000(t *testing.T) {
	m := loadManifest(t)
	got := ArchCandidates("aarch64_cortex-a73", m.Architectures)
	if len(got) != 2 || got[0] != "aarch64_cortex-a53" || got[1] != "aarch64_generic" {
		t.Fatalf("got %v", got)
	}
	if got := ArchCandidates("mipsel_24kc", m.Architectures); len(got) != 1 || got[0] != "mipsel_24kc" {
		t.Fatalf("mipsel: got %v", got)
	}
}

func TestSelectAssets(t *testing.T) {
	m := loadManifest(t)
	archs := ArchCandidates("aarch64_cortex-a73", m.Architectures)
	pkgs := []string{"hybrid-failover-core", "luci-i18n-hybrid-failover", "luci-app-hybrid-failover", "hybrid-failover-bot"}

	apk, err := SelectAssets(m, "apk", archs, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if apk["hybrid-failover-core"] != "hybrid-failover-core-1.7.45-r1_aarch64_cortex-a53.apk" ||
		apk["luci-app-hybrid-failover"] != "luci-app-hybrid-failover-1.7.45-r1.apk" {
		t.Fatalf("apk: %v", apk)
	}
	ipk, err := SelectAssets(m, "opkg", archs, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if ipk["hybrid-failover-core"] != "hybrid-failover-core_1.7.45-1_aarch64_cortex-a53.ipk" ||
		ipk["luci-app-hybrid-failover"] != "luci-app-hybrid-failover_1.7.45-1_all.ipk" {
		t.Fatalf("ipk: %v", ipk)
	}
	if _, err := SelectAssets(m, "apk", []string{"riscv64"}, []string{"hybrid-failover-core"}); err == nil {
		t.Fatal("missing architecture must fail")
	}
}

// fakeGitHub serves releases/latest, manifest.json and the package files of
// one release, with the manifest describing them correctly unless corrupt is set.
func fakeGitHub(t *testing.T, tag string, files map[string][]byte, corrupt string) *httptest.Server {
	t.Helper()
	m := Manifest{Version: strings.TrimPrefix(tag, "v") + "-1", APKVersion: strings.TrimPrefix(tag, "v") + "-r1",
		PkgFormat: "both", Architectures: []string{"aarch64_cortex-a53", "aarch64_generic"}, Packages: map[string]Asset{}}
	for name, body := range files {
		sum := sha256.Sum256(body)
		m.Packages[name] = Asset{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Release{Tag: tag, Body: "notes", HTMLURL: "https://example/" + tag})
	})
	mux.HandleFunc("/web/o/r/releases/download/"+tag+"/", func(w http.ResponseWriter, r *http.Request) {
		name := filepath.Base(r.URL.Path)
		if name == "manifest.json" {
			json.NewEncoder(w).Encode(m)
			return
		}
		body, ok := files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if name == corrupt {
			body = append([]byte("x"), body[1:]...)
		}
		w.Write(body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testClient(t *testing.T, srv *httptest.Server) *Client {
	return &Client{Repo: "o/r", APIBase: srv.URL + "/api", WebBase: srv.URL + "/web", StateDir: t.TempDir()}
}

func releaseFiles() map[string][]byte {
	return map[string][]byte{
		"hybrid-failover-core-1.8.0-r1_aarch64_cortex-a53.apk": []byte("core-binary"),
		"luci-i18n-hybrid-failover-1.8.0-r1.apk":               []byte("i18n"),
		"luci-app-hybrid-failover-1.8.0-r1.apk":                []byte("luci"),
		"hybrid-failover-bot-1.8.0-r1_aarch64_cortex-a53.apk":  []byte("bot"),
	}
}

type recorder struct {
	mu    sync.Mutex
	calls []string
	have  map[string]bool
}

func (r *recorder) run(ctx context.Context, name string, args ...string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	if name == "apk" && len(args) == 3 && args[0] == "info" && args[1] == "-e" {
		if r.have[args[2]] {
			return "", nil
		}
		return "", fmt.Errorf("not installed")
	}
	return "", nil
}

func TestCheck(t *testing.T) {
	srv := fakeGitHub(t, "v1.8.0", releaseFiles(), "")
	c := testClient(t, srv)
	res := c.Check(context.Background(), "1.7.45")
	if res.Error != "" || !res.Available || res.Latest != "1.8.0" || res.Tag != "v1.8.0" {
		t.Fatalf("check: %+v", res)
	}
	if cached, ok := c.CachedCheck(); !ok || cached.Latest != "1.8.0" {
		t.Fatalf("cache: %+v %v", cached, ok)
	}
	if res := c.Check(context.Background(), "1.8.0"); res.Available {
		t.Fatal("same version must not be offered")
	}
}

func TestApplyInstallsVerifiedPackages(t *testing.T) {
	srv := fakeGitHub(t, "v1.8.0", releaseFiles(), "")
	c := testClient(t, srv)
	rec := &recorder{have: map[string]bool{"hybrid-failover-core": true, "hybrid-failover-bot": false}}
	env := &Env{PM: "apk", DistribArch: "aarch64_cortex-a73", Run: rec.run}
	if err := c.Apply(context.Background(), env, ApplyOptions{Installed: "1.7.45"}); err != nil {
		t.Fatal(err)
	}
	var install string
	for _, call := range rec.calls {
		if strings.HasPrefix(call, "apk add") {
			install = call
		}
	}
	for _, want := range []string{"--allow-untrusted", "hybrid-failover-core-1.8.0-r1_aarch64_cortex-a53.apk", "luci-app-hybrid-failover-1.8.0-r1.apk", "luci-i18n-hybrid-failover-1.8.0-r1.apk"} {
		if !strings.Contains(install, want) {
			t.Fatalf("install call %q lacks %q", install, want)
		}
	}
	if strings.Contains(install, "hybrid-failover-bot") {
		t.Fatalf("bot is not installed and must not be added: %q", install)
	}
	st := c.ReadState()
	if st.State != "done" || st.To != "1.8.0" {
		t.Fatalf("state: %+v", st)
	}
}

func TestApplyRejectsCorruptDownload(t *testing.T) {
	srv := fakeGitHub(t, "v1.8.0", releaseFiles(), "luci-app-hybrid-failover-1.8.0-r1.apk")
	c := testClient(t, srv)
	rec := &recorder{have: map[string]bool{}}
	env := &Env{PM: "apk", DistribArch: "aarch64_cortex-a73", Run: rec.run}
	err := c.Apply(context.Background(), env, ApplyOptions{Installed: "1.7.45"})
	if err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("want sha256 error, got %v", err)
	}
	for _, call := range rec.calls {
		if strings.HasPrefix(call, "apk add") {
			t.Fatalf("nothing may be installed after a bad download: %q", call)
		}
	}
	if st := c.ReadState(); st.State != "failed" {
		t.Fatalf("state: %+v", st)
	}
}

func TestApplyRefusesOlderUnlessForced(t *testing.T) {
	srv := fakeGitHub(t, "v1.8.0", releaseFiles(), "")
	c := testClient(t, srv)
	rec := &recorder{have: map[string]bool{}}
	env := &Env{PM: "apk", DistribArch: "aarch64_cortex-a73", Run: rec.run}
	if err := c.Apply(context.Background(), env, ApplyOptions{Installed: "1.8.0"}); err == nil {
		t.Fatal("same version must be refused")
	}
	if err := c.Apply(context.Background(), env, ApplyOptions{Installed: "1.8.0", Force: true}); err != nil {
		t.Fatalf("forced: %v", err)
	}
}
