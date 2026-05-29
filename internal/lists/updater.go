package lists

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
)

type Updater struct {
	RulesetDir string
	ViaProxy   bool
	HTTP       *http.Client
	mu         sync.Mutex
	running    bool
}

func NewUpdater(viaProxy bool) *Updater {
	return &Updater{
		RulesetDir: singbox.RulesetDir,
		ViaProxy:   viaProxy,
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
}

func (u *Updater) UpdateOnce() error {
	u.mu.Lock()
	if u.running {
		u.mu.Unlock()
		return fmt.Errorf("list update already running")
	}
	u.running = true
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.running = false
		u.mu.Unlock()
	}()

	if err := os.MkdirAll(u.RulesetDir, 0o755); err != nil {
		return err
	}
	for _, svc := range singbox.CommunityServices {
		if err := u.fetchCommunityDomains(svc); err != nil {
			return err
		}
		if subnetURL, ok := singbox.SubnetListURLs[svc]; ok {
			if err := u.fetchURL(subnetURL, svc+".lst"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *Updater) fetchCommunityDomains(service string) error {
	url := singbox.CommunityServiceDomainURL(service)
	return u.fetchURL(url, service+".txt")
}

func (u *Updater) fetchURL(url, filename string) error {
	resp, err := u.HTTP.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	dest := filepath.Join(u.RulesetDir, filename)
	return os.WriteFile(dest, body, 0o644)
}

func WritePID() error {
	return os.WriteFile(paths.ListUpdatePID, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func ClearPID() {
	_ = os.Remove(paths.ListUpdatePID)
}
