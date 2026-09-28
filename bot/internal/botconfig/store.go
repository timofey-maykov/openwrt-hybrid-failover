package botconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
)

type Store struct {
	activePath  string
	pendingPath string
}

func NewStore(activePath string) Store {
	return Store{
		activePath:  activePath,
		pendingPath: activePath + ".pending",
	}
}

func (s Store) LoadActive() (config.Config, error) {
	return config.Load(s.activePath)
}

func (s Store) LoadPending() (config.Config, error) {
	_, err := os.Stat(s.pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.LoadActive()
		}
		return config.Config{}, err
	}
	return config.Load(s.pendingPath)
}

func (s Store) SetPendingKey(key, value string) error {
	cfg, err := s.LoadPending()
	if err != nil {
		return err
	}

	switch key {
	case "policy":
		cfg.Policy = value
	case "log_path":
		cfg.LogPath = value
	case "audit_path":
		cfg.AuditPath = value
	case "clash_api":
		cfg.ClashAPI = value
	case "routing_init_script":
		cfg.RoutingInitScript = value
	case "token":
		cfg.Token = value
	case "router_name":
		cfg.RouterName = strings.TrimSpace(value)
	case "admin_ids":
		ids, err := parseAdminIDs(value)
		if err != nil {
			return err
		}
		cfg.AdminIDs = ids
	case "viewer_ids":
		ids, err := parseIDs(value, true)
		if err != nil {
			return err
		}
		cfg.ViewerIDs = ids
	case "notify_failover_enabled":
		on, err := parseBool(value)
		if err != nil {
			return err
		}
		cfg.NotifyFailoverEnabled = on
	case "notify_failover_interval_seconds":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 10 {
			return fmt.Errorf("notify_failover_interval_seconds must be an integer >= 10")
		}
		cfg.NotifyFailoverIntervalSeconds = n
	case "probe_timeout_seconds":
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n <= 0 {
			return fmt.Errorf("probe_timeout_seconds must be positive integer")
		}
		cfg.ProbeTimeoutSeconds = n
	default:
		return fmt.Errorf("key %q is not editable", key)
	}
	return s.writeJSON(s.pendingPath, cfg)
}

func (s Store) ValidatePending() error {
	cfg, err := s.LoadPending()
	if err != nil {
		return err
	}
	return cfg.Validate()
}

func (s Store) ApplyPending() error {
	if err := s.ValidatePending(); err != nil {
		return err
	}
	raw, err := os.ReadFile(s.pendingPath)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.activePath, raw, 0o600); err != nil {
		return err
	}
	return os.Remove(s.pendingPath)
}

func (s Store) RollbackPending() error {
	if err := os.Remove(s.pendingPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s Store) DiffSummary() (string, error) {
	activeRaw, err := os.ReadFile(s.activePath)
	if err != nil {
		return "", err
	}
	pendingRaw, err := os.ReadFile(s.pendingPath)
	if err != nil {
		return "", err
	}
	if string(activeRaw) == string(pendingRaw) {
		return "no changes", nil
	}
	return strings.Join([]string{
		"pending config differs from active config",
		fmt.Sprintf("active: %s", filepath.Base(s.activePath)),
		fmt.Sprintf("pending: %s", filepath.Base(s.pendingPath)),
	}, "\n"), nil
}

func (s Store) writeJSON(path string, cfg config.Config) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "on", "yes":
		return true, nil
	case "0", "false", "off", "no", "":
		return false, nil
	}
	return false, fmt.Errorf("expected true/false, got %q", raw)
}

func parseAdminIDs(raw string) ([]int64, error) {
	return parseIDs(raw, false)
}

// parseIDs parses a comma or space separated list of Telegram user IDs.
func parseIDs(raw string, allowEmpty bool) ([]int64, error) {
	raw = strings.ReplaceAll(raw, " ", ",")
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		n, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid user id %q", trimmed)
		}
		out = append(out, n)
	}
	if len(out) == 0 && !allowEmpty {
		return nil, fmt.Errorf("admin_ids cannot be empty")
	}
	return out, nil
}
