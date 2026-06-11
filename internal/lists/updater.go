package lists

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	return &Updater{
		RulesetDir: singbox.RulesetDir,
		UCIPath:    paths.UCIConfig,
		ViaProxy:   viaProxy,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
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
	for _, svc := range services {
		subnetURL, ok := singbox.SubnetListURLs[svc]
		if !ok {
			continue
		}
		changed, err := u.fetchSubnetList(subnetURL, svc+".lst")
		if err != nil {
			return result, err
		}
		result.Changed = result.Changed || changed
	}
	return result, nil
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
