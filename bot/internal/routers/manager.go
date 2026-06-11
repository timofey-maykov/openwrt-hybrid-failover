package routers

import (
	"fmt"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerexec"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
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
}

func NewManager(cfg config.Config) (*Manager, error) {
	m := &Manager{
		instances: map[string]*Instance{},
		selection: map[int64]string{},
	}
	timeout := cfg.ProbeTimeoutSeconds
	if timeout <= 0 {
		timeout = 5
	}
	dur := cfg.ProbeDuration()

	if len(cfg.Routers) == 0 {
		svc := routing.NewService(
			routerexec.NewLocal(dur),
			cfg.ClashAPI,
			cfg.RoutingInitScript,
			cfg.UCIPackage,
			cfg.MainSection,
			dur,
		)
		m.instances["local"] = &Instance{ID: "local", Name: "local", Service: svc}
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
			exec = routerexec.NewLocal(dur)
		} else {
			if rc.Host == "" {
				return nil, fmt.Errorf("router %q: host is required unless local=true", rc.ID)
			}
			if rc.IdentityFile == "" {
				return nil, fmt.Errorf("router %q: identity_file is required for remote router", rc.ID)
			}
			exec = routerexec.NewSSH(dur, rc.Host, rc.Port, rc.User, rc.IdentityFile)
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
		svc := routing.NewService(exec, clashAPI, initScript, uciPkg, mainSec, dur)
		m.instances[rc.ID] = &Instance{ID: rc.ID, Name: name, Service: svc}
		m.order = append(m.order, rc.ID)
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
