// Package selfupdate checks GitHub for a newer release of this project and
// installs it on the router from the release's own packages.
//
// The release publishes one manifest.json with the sha256 and size of every
// package file (apk and ipk, per architecture). Apply picks the files for the
// router's package manager and architecture, downloads only the packages
// that make sense here (core and LuCI always, the bot and curfew only if they
// are installed), checks every file against the manifest and installs them in
// one package-manager call. Progress goes to a small JSON state file so LuCI
// can follow an update that restarts rpcd underneath it.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo  = "timofey-maykov/openwrt-hybrid-failover"
	DefaultAPI   = "https://api.github.com"
	DefaultWeb   = "https://github.com"
	DefaultState = "/tmp/hybrid-failover/update"
)

// Packages this project ships. Core, LuCI and its translations are always
// part of an update; the others only when already installed.
var (
	basePackages     = []string{"hybrid-failover-core", "luci-i18n-hybrid-failover", "luci-app-hybrid-failover"}
	optionalPackages = []string{"hybrid-failover-bot", "curfew", "luci-app-curfew"}
)

// Release is the part of the GitHub release API this package uses.
type Release struct {
	Tag         string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

// CheckResult is what `update check` prints and caches.
type CheckResult struct {
	Installed   string `json:"installed"`
	Latest      string `json:"latest"`
	Tag         string `json:"tag"`
	Available   bool   `json:"available"`
	PublishedAt string `json:"published_at,omitempty"`
	Notes       string `json:"notes,omitempty"`
	URL         string `json:"url,omitempty"`
	CheckedAt   int64  `json:"checked_at"`
	Error       string `json:"error,omitempty"`
}

// Asset is one manifest entry.
type Asset struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest is the release's manifest.json.
type Manifest struct {
	Version       string           `json:"version"`
	APKVersion    string           `json:"apk_version"`
	PkgFormat     string           `json:"pkg_format"`
	Architectures []string         `json:"architectures"`
	Packages      map[string]Asset `json:"packages"`
}

// Client talks to GitHub. Zero fields fall back to the defaults.
type Client struct {
	HTTP     *http.Client
	Repo     string
	APIBase  string
	WebBase  string
	StateDir string
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return DefaultRepo
}

func (c *Client) api() string {
	if c.APIBase != "" {
		return strings.TrimRight(c.APIBase, "/")
	}
	return DefaultAPI
}

func (c *Client) web() string {
	if c.WebBase != "" {
		return strings.TrimRight(c.WebBase, "/")
	}
	return DefaultWeb
}

func (c *Client) stateDir() string {
	if c.StateDir != "" {
		return c.StateDir
	}
	return DefaultState
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	var last error
	// Downloads from GitHub get reset mid-way now and then on some ISPs;
	// a few tries cost nothing.
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "hybrid-failover-selfupdate")
		req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream, */*")
		resp, err := c.http().Do(req)
		if err != nil {
			last = err
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		last = fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
		if resp.StatusCode == http.StatusNotFound {
			break
		}
	}
	return nil, last
}

// LatestRelease returns the newest published, non-draft, non-prerelease release.
func (c *Client) LatestRelease(ctx context.Context) (Release, error) {
	var rel Release
	resp, err := c.get(ctx, c.api()+"/repos/"+c.repo()+"/releases/latest")
	if err != nil {
		return rel, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return rel, fmt.Errorf("release JSON: %w", err)
	}
	if rel.Tag == "" {
		return rel, errors.New("release has no tag")
	}
	return rel, nil
}

// Manifest downloads and parses manifest.json of a release.
func (c *Client) Manifest(ctx context.Context, tag string) (Manifest, error) {
	var m Manifest
	resp, err := c.get(ctx, c.web()+"/"+c.repo()+"/releases/download/"+tag+"/manifest.json")
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&m); err != nil {
		return m, fmt.Errorf("manifest.json: %w", err)
	}
	if len(m.Packages) == 0 {
		return m, errors.New("manifest.json lists no packages")
	}
	return m, nil
}

// Check compares the installed version with the latest release and caches
// the result in the state directory.
func (c *Client) Check(ctx context.Context, installed string) CheckResult {
	res := CheckResult{Installed: installed, CheckedAt: time.Now().Unix()}
	rel, err := c.LatestRelease(ctx)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Tag = rel.Tag
		res.Latest = strings.TrimPrefix(rel.Tag, "v")
		res.PublishedAt = rel.PublishedAt
		res.Notes = rel.Body
		res.URL = rel.HTMLURL
		res.Available = Newer(res.Latest, installed)
	}
	_ = c.writeJSON("check.json", res)
	return res
}

// CachedCheck returns the last Check result, if any.
func (c *Client) CachedCheck() (CheckResult, bool) {
	var res CheckResult
	data, err := os.ReadFile(filepath.Join(c.stateDir(), "check.json"))
	if err != nil || json.Unmarshal(data, &res) != nil {
		return res, false
	}
	return res, true
}

func (c *Client) writeJSON(name string, v any) error {
	if err := os.MkdirAll(c.stateDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(c.stateDir(), name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(c.stateDir(), name))
}

// ParseVersion reads "v1.7.45", "1.7.45-1" or "1.7.45-pprof" as [1 7 45].
func ParseVersion(s string) ([3]int, bool) {
	var v [3]int
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexFunc(s, func(r rune) bool { return (r < '0' || r > '9') && r != '.' }); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || parts[0] == "" {
		return v, false
	}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

// Newer reports whether latest is a higher version than installed. An
// installed version that cannot be parsed (a "dev" build) never gets an
// update offered, so hand-made builds are not replaced by surprise.
func Newer(latest, installed string) bool {
	l, ok1 := ParseVersion(latest)
	i, ok2 := ParseVersion(installed)
	if !ok1 || !ok2 {
		return false
	}
	for k := 0; k < 3; k++ {
		if l[k] != i[k] {
			return l[k] > i[k]
		}
	}
	return false
}

// ArchCandidates turns the router's DISTRIB_ARCH into the package
// architectures to try, best first, limited to what the release ships.
func ArchCandidates(distribArch string, shipped []string) []string {
	var cands []string
	add := func(a string) {
		for _, x := range cands {
			if x == a {
				return
			}
		}
		cands = append(cands, a)
	}
	a := strings.TrimSpace(distribArch)
	add(a)
	switch {
	case strings.HasPrefix(a, "aarch64"):
		add("aarch64_cortex-a53")
		add("aarch64_generic")
	case strings.HasPrefix(a, "arm"):
		add("arm_cortex-a7")
	case strings.HasPrefix(a, "mipsel"):
		add("mipsel_24kc")
	case strings.HasPrefix(a, "mips"):
		add("mips_24kc")
	case strings.HasPrefix(a, "x86_64"):
		add("x86_64")
	}
	if len(shipped) == 0 {
		return cands
	}
	var out []string
	for _, c := range cands {
		for _, s := range shipped {
			if c == s {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// SelectAssets maps each package to the manifest file for this package
// manager ("apk" or "opkg") and architecture.
func SelectAssets(m Manifest, pm string, archs []string, pkgs []string) (map[string]string, error) {
	out := make(map[string]string, len(pkgs))
	for _, pkg := range pkgs {
		var names []string
		switch pm {
		case "apk":
			for _, a := range archs {
				names = append(names, fmt.Sprintf("%s-%s_%s.apk", pkg, m.APKVersion, a))
			}
			names = append(names, fmt.Sprintf("%s-%s.apk", pkg, m.APKVersion))
		case "opkg":
			for _, a := range archs {
				names = append(names, fmt.Sprintf("%s_%s_%s.ipk", pkg, m.Version, a))
			}
			names = append(names, fmt.Sprintf("%s_%s_all.ipk", pkg, m.Version))
		default:
			return nil, fmt.Errorf("unknown package manager %q", pm)
		}
		found := ""
		for _, n := range names {
			if _, ok := m.Packages[n]; ok {
				found = n
				break
			}
		}
		if found == "" {
			return nil, fmt.Errorf("%s: no %s package for %s in the release", pkg, pm, strings.Join(archs, "/"))
		}
		out[pkg] = found
	}
	return out, nil
}

// Download fetches one release file and checks it against the manifest.
func (c *Client) Download(ctx context.Context, tag, name string, want Asset, dir string) (string, error) {
	resp, err := c.get(ctx, c.web()+"/"+c.repo()+"/releases/download/"+tag+"/"+name)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 64<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
		return "", err
	}
	if want.Size > 0 && n != want.Size {
		os.Remove(path)
		return "", fmt.Errorf("%s: size %d, manifest says %d", name, n, want.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, want.SHA256) {
		os.Remove(path)
		return "", fmt.Errorf("%s: sha256 %s, manifest says %s", name, got, want.SHA256)
	}
	return path, nil
}

// SortedPackages lists the packages to update: the base set first, then the
// optional ones that isInstalled reports as present.
func SortedPackages(isInstalled func(string) bool) []string {
	pkgs := append([]string(nil), basePackages...)
	var extra []string
	for _, p := range optionalPackages {
		if isInstalled(p) {
			extra = append(extra, p)
		}
	}
	sort.Strings(extra)
	return append(pkgs, extra...)
}
