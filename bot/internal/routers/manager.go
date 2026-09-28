package routers

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerexec"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

type Instance struct {
	ID      string
	Name    string
	Service routing.Service
}

type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	order     []string
	selection map[int64]string
	// Warnings lists config problems worked around at startup (for the log).
	Warnings []string
}

// Seams for tests: the local UCI lookup and the key file check.
var (
	localUCIGet = func(key string) (string, bool) {
		out, err := exec.Command("uci", "-q", "get", key).Output()
		if err != nil {
			return "", false
		}
		return strings.TrimSpace(string(out)), true
	}
	fileExists = func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
)

// localMainSection checks the configured section against this router's UCI.
// The shipped example config used main_section "main" while core defaults to
// "glob"; with a missing section every UCI key the bot touches is wrong. Core's
// settings.main_section is authoritative when the configured one is absent.
func localMainSection(pkg, configured string) (string, string) {
	if _, ok := localUCIGet(pkg + "." + configured); ok {
		return configured, ""
	}
	if _, ok := localUCIGet(pkg + ".settings"); !ok {
		return configured, "" // no UCI here (tests, bot on a non-OpenWrt host)
	}
	actual := paths.DefaultMainSection
	if v, ok := localUCIGet(pkg + ".settings.main_section"); ok && v != "" {
		actual = v
	}
	if actual == configured {
		return configured, ""
	}
	return actual, fmt.Sprintf("main_section %q не найдена в UCI %s, использую %q", configured, pkg, actual)
}

func NewManager(cfg config.Config) (*Manager, error) {
	m := &Manager{
		instances: map[string]*Instance{},
		selection: map[int64]string{},
	}
	// dur bounds HTTP probes; router commands get their own, longer budget
	// (uci, core RPC and init.d calls routinely take more than a probe timeout).
	dur := cfg.ProbeDuration()
	cmdTimeout := routerexec.DefaultCommandTimeout

	if len(cfg.Routers) == 0 {
		mainSec, warn := localMainSection(cfg.UCIPackage, cfg.MainSection)
		if warn != "" {
			m.Warnings = append(m.Warnings, warn)
		}
		svc := routing.NewService(
			routerexec.NewLocal(cmdTimeout),
			cfg.ClashAPI,
			cfg.RoutingInitScript,
			cfg.UCIPackage,
			mainSec,
			dur,
		)
		m.instances["local"] = &Instance{ID: "local", Name: cfg.Identity(), Service: svc}
		m.order = []string{"local"}
		return m, nil
	}

	for _, rc := range cfg.Routers {
		if rc.ID == "" {
			return nil, fmt.Errorf("router: missing id")
		}
		if _, exists := m.instances[rc.ID]; exists {
			return nil, fmt.Errorf("router %q: duplicate id", rc.ID)
		}
		name := rc.Name
		if name == "" {
			name = rc.ID
		}
		var exec routerexec.Exec
		if rc.Local {
			exec = routerexec.NewLocal(cmdTimeout)
		} else {
			if rc.Host == "" {
				return nil, fmt.Errorf("router %q: host is required unless local=true", rc.ID)
			}
			if rc.IdentityFile == "" {
				return nil, fmt.Errorf("router %q: identity_file is required for remote router", rc.ID)
			}
			if !fileExists(rc.IdentityFile) {
				// Without the key every command fails, and with two entries the
				// user must /use a router before anything works. Skip it so a
				// bot left with one router keeps managing that router.
				m.Warnings = append(m.Warnings, fmt.Sprintf("роутер %q пропущен: нет ключа %s", rc.ID, rc.IdentityFile))
				continue
			}
			exec = routerexec.NewSSH(cmdTimeout, rc.Host, rc.Port, rc.User, rc.IdentityFile)
		}
		clashAPI := rc.ClashAPI
		if clashAPI == "" {
			clashAPI = cfg.ClashAPI
		}
		initScript := rc.RoutingInitScript
		if initScript == "" {
			initScript = cfg.RoutingInitScript
		}
		uciPkg := rc.UCIPackage
		if uciPkg == "" {
			uciPkg = cfg.UCIPackage
		}
		mainSec := rc.MainSection
		if mainSec == "" {
			mainSec = cfg.MainSection
		}
		if rc.Local {
			sec, warn := localMainSection(uciPkg, mainSec)
			if warn != "" {
				m.Warnings = append(m.Warnings, fmt.Sprintf("роутер %q: %s", rc.ID, warn))
			}
			mainSec = sec
		}
		svc := routing.NewService(exec, clashAPI, initScript, uciPkg, mainSec, dur)
		m.instances[rc.ID] = &Instance{ID: rc.ID, Name: name, Service: svc}
		m.order = append(m.order, rc.ID)
	}
	if len(m.order) == 0 {
		return nil, fmt.Errorf("нет доступных роутеров: %s", strings.Join(m.Warnings, "; "))
	}
	return m, nil
}

func (m *Manager) List() []Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Instance, 0, len(m.order))
	for _, id := range m.order {
		if inst, ok := m.instances[id]; ok {
			out = append(out, *inst)
		}
	}
	return out
}

func (m *Manager) DefaultID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.order) == 0 {
		return "local"
	}
	return m.order[0]
}

func (m *Manager) SelectedID(userID int64) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectedIDLocked(userID)
}

func (m *Manager) selectedIDLocked(userID int64) string {
	if id, ok := m.selection[userID]; ok && id != "" {
		if _, exists := m.instances[id]; exists {
			return id
		}
	}
	if len(m.order) == 1 {
		return m.order[0]
	}
	return ""
}

func (m *Manager) SetSelected(userID int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.instances[id]; !ok {
		return fmt.Errorf("роутер %q не найден", id)
	}
	m.selection[userID] = id
	return nil
}

func (m *Manager) InstanceFor(userID int64) (*Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id := m.selectedIDLocked(userID)
	if id == "" {
		return nil, fmt.Errorf("роутер не выбран: /routers и /use <id>")
	}
	inst, ok := m.instances[id]
	if !ok {
		return nil, fmt.Errorf("роутер %q не найден", id)
	}
	return inst, nil
}

func (m *Manager) Prefix(userID int64) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.instances) <= 1 {
		return ""
	}
	id := m.selectedIDLocked(userID)
	if id == "" {
		return "[?] "
	}
	if inst, ok := m.instances[id]; ok {
		return fmt.Sprintf("[%s] ", inst.Name)
	}
	return ""
}

func (m *Manager) Multi() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances) > 1
}
