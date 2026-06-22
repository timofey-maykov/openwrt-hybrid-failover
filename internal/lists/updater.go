package lists

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/netfetch"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type Updater struct {
	RulesetDir string
	UCIPath    string
	ViaProxy   bool
	HTTP       *http.Client
	mu         sync.Mutex
	running    bool
}

type UpdateResult struct {
	Changed bool
}

func NewUpdater(viaProxy bool) *Updater {
	u := &Updater{
		RulesetDir: singbox.RulesetDir,
		UCIPath:    paths.UCIConfig,
		ViaProxy:   viaProxy,
	}
	u.HTTP = u.httpClient(viaProxy)
	return u
}

// NewFromUCI builds an updater using hybrid-failover.settings list download options.
func NewFromUCI(uciPath string) *Updater {
	if uciPath == "" {
		uciPath = paths.UCIConfig
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return NewUpdater(false)
	}
	enabled, section := singbox.ListDownloadSection(pkg)
	u := &Updater{
		RulesetDir: singbox.RulesetDir,
		UCIPath:    uciPath,
		ViaProxy:   enabled && section != "",
	}
	u.HTTP = u.httpClient(u.ViaProxy)
	return u
}

func (u *Updater) httpClient(viaProxy bool) *http.Client {
	dnsAddr := u.resolverDNS(viaProxy)
	proxyURL := ""
	if viaProxy {
		proxyURL = singbox.ListDownloadProxyURL
	}
	return netfetch.HTTPClient(dnsAddr, proxyURL, 60*time.Second)
}

func (u *Updater) resolverDNS(viaProxy bool) string {
	if viaProxy {
		return netfetch.SingboxDNSAddr
	}
	pkg, err := uci.Load(u.UCIPath)
	if err != nil {
		return netfetch.DefaultDNSAddr
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return netfetch.DefaultDNSAddr
	}
	if boot := strings.TrimSpace(settings.Get("bootstrap_dns_server", "")); boot != "" {
		return net.JoinHostPort(boot, "53")
	}
	return netfetch.DefaultDNSAddr
}

func (u *Updater) UpdateOnce() (UpdateResult, error) {
	u.mu.Lock()
	if u.running {
		u.mu.Unlock()
		return UpdateResult{}, fmt.Errorf("list update already running")
	}
	u.running = true
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.running = false
		u.mu.Unlock()
	}()

	services := u.configuredServices()
	if len(services) == 0 {
		return UpdateResult{}, nil
	}

	if err := os.MkdirAll(u.RulesetDir, 0o755); err != nil {
		return UpdateResult{}, err
	}

	var result UpdateResult
	changed, err := u.syncCommunityDomainRulesets()
	if err != nil {
		return result, err
	}
	result.Changed = result.Changed || changed
	for _, svc := range services {
		subnetURL, ok := subnetListURL(svc)
		if !ok {
			continue
		}
		filename := svc + ".lst"
		changed, err := u.fetchSubnetList(subnetURL, filename)
		if err != nil {
			if u.cachedListValid(filename) {
				continue
			}
			return result, err
		}
		result.Changed = result.Changed || changed
	}
	return result, nil
}

// HasValidCache reports whether configured community lists have usable local cache.
func (u *Updater) HasValidCache() bool {
	pkg, err := uci.Load(u.UCIPath)
	if err != nil {
		return false
	}
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil || !singbox.SectionHasEnabledLists(sec) {
			continue
		}
		for _, svc := range sec.GetList("community_lists") {
			svc = strings.TrimSpace(svc)
			if svc == "" {
				continue
			}
			tag := singbox.RulesetTag(name, svc, "community")
			path := filepath.Join(u.RulesetDir, tag+".json")
			if singbox.DomainRulesetEmpty(path) {
				return false
			}
			if url, ok := subnetListURL(svc); ok && url != "" {
				if !u.cachedListValid(svc + ".lst") {
					return false
				}
			}
		}
	}
	return true
}

func (u *Updater) cachedListValid(filename string) bool {
	path := filepath.Join(u.RulesetDir, filename)
	cidrs, err := subnets.ParseFile(path)
	return err == nil && len(cidrs) > 0
}

func (u *Updater) configuredServices() []string {
	pkg, err := uci.Load(u.UCIPath)
	if err != nil {
		return singbox.CommunityServices
	}
	seen := make(map[string]struct{})
	var out []string
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil || !singbox.SectionHasEnabledLists(sec) {
			continue
		}
		for _, svc := range sec.GetList("community_lists") {
			svc = strings.TrimSpace(svc)
			if svc == "" {
				continue
			}
			if _, ok := seen[svc]; ok {
				continue
			}
			seen[svc] = struct{}{}
			out = append(out, svc)
		}
	}
	return out
}

func (u *Updater) syncCommunityDomainRulesets() (bool, error) {
	pkg, err := uci.Load(u.UCIPath)
	if err != nil {
		return false, err
	}
	var changed bool
	var fetchErrs []string
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil || !singbox.SectionHasEnabledLists(sec) {
			continue
		}
		for _, svc := range sec.GetList("community_lists") {
			svc = strings.TrimSpace(svc)
			if svc == "" {
				continue
			}
			tag := singbox.RulesetTag(name, svc, "community")
			path := filepath.Join(u.RulesetDir, tag+".json")
			if singbox.DomainRulesetEmpty(path) {
				singbox.EnsureSourceRuleset(path)
			}
			url := singbox.CommunityServiceDomainURL(svc)
			c, err := u.fetchDomainRuleset(url, path)
			if err != nil {
				if singbox.DomainRulesetEmpty(path) {
					fetchErrs = append(fetchErrs, fmt.Sprintf("%s: %v", tag, err))
				}
				continue
			}
			changed = changed || c
		}
	}
	if len(fetchErrs) > 0 {
		return changed, fmt.Errorf("community domain rulesets: %s", strings.Join(fetchErrs, "; "))
	}
	return changed, nil
}

func (u *Updater) fetchDomainRuleset(url, path string) (bool, error) {
	resp, err := u.HTTP.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	domains := singbox.ParseDomainListBody(string(body))
	if len(domains) == 0 {
		return false, nil
	}
	prev, _ := os.ReadFile(path)
	if err := singbox.WriteDomainRuleset(path, domains); err != nil {
		return false, err
	}
	next, _ := os.ReadFile(path)
	return !bytesEqual(prev, next), nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (u *Updater) fetchSubnetList(url, filename string) (bool, error) {
	resp, err := u.HTTP.Get(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	dest := filepath.Join(u.RulesetDir, filename)
	changed, err := subnets.WriteListBodyIfChanged(dest, body)
	if err != nil {
		return false, fmt.Errorf("fetch %s: %w", url, err)
	}
	return changed, nil
}

func WritePID() error {
	return os.WriteFile(paths.ListUpdatePID, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func ClearPID() {
	_ = os.Remove(paths.ListUpdatePID)
}

var subnetListURL = func(service string) (string, bool) {
	url, ok := singbox.SubnetListURLs[service]
	return url, ok
}
