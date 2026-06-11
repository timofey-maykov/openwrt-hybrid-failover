package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

type Config struct {
	Token                         string         `json:"token"`
	AdminIDs                      []int64        `json:"admin_ids"`
	ViewerIDs                     []int64        `json:"viewer_ids"`
	LogPath                       string         `json:"log_path"`
	AuditPath                     string         `json:"audit_path"`
	ClashAPI                      string         `json:"clash_api"`
	RoutingInitScript             string         `json:"routing_init_script"`
	UCIPackage                    string         `json:"uci_package"`
	MainSection                   string         `json:"main_section"`
	Policy                        string         `json:"policy"`
	ProbeTimeoutSeconds           int            `json:"probe_timeout_seconds"`
	NotifyFailoverEnabled         bool           `json:"notify_failover_enabled"`
	NotifyFailoverIntervalSeconds int            `json:"notify_failover_interval_seconds"`
	Routers                       []RouterConfig `json:"routers"`
}

// RouterConfig is one managed OpenWrt host. Remote routers use SSH (key in identity_file).
type RouterConfig struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Local             bool   `json:"local"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	User              string `json:"user"`
	IdentityFile      string `json:"identity_file"`
	ClashAPI          string `json:"clash_api"`
	RoutingInitScript string `json:"routing_init_script"`
	UCIPackage        string `json:"uci_package"`
	MainSection       string `json:"main_section"`
}

func (c Config) ProbeDuration() time.Duration {
	sec := c.ProbeTimeoutSeconds
	if sec <= 0 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var legacy map[string]any
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	migrateLegacy(legacy, &cfg)
	if envToken := strings.TrimSpace(os.Getenv("HF_BOT_TOKEN")); envToken != "" {
		cfg.Token = envToken
	}
	if cfg.Policy == "" {
		cfg.Policy = "outage-only"
	}
	if cfg.ProbeTimeoutSeconds <= 0 {
		cfg.ProbeTimeoutSeconds = 5
	}
	if cfg.NotifyFailoverIntervalSeconds <= 0 {
		cfg.NotifyFailoverIntervalSeconds = 30
	}
	if cfg.ClashAPI == "" {
		cfg.ClashAPI = "http://127.0.0.1:9090"
	}
	if cfg.RoutingInitScript == "" {
		cfg.RoutingInitScript = paths.CoreInit
	}
	if cfg.UCIPackage == "" {
		cfg.UCIPackage = paths.UCIPackage
	}
	if cfg.MainSection == "" {
		cfg.MainSection = paths.DefaultMainSection
	}
	return cfg, cfg.Validate()
}

func migrateLegacy(legacy map[string]any, cfg *Config) {
	if cfg.RoutingInitScript == "" {
		if v, ok := legacyString(legacy, "podkop_init_script"); ok {
			cfg.RoutingInitScript = v
		}
	}
	cfg.RoutingInitScript = strings.ReplaceAll(cfg.RoutingInitScript, "/etc/init.d/podkop", paths.CoreInit)
	cfg.LogPath = strings.ReplaceAll(cfg.LogPath, "podkop-telegram-bot", "hybrid-failover-bot")
	cfg.AuditPath = strings.ReplaceAll(cfg.AuditPath, "podkop-telegram-bot", "hybrid-failover-bot")
	if cfg.LogPath == "" {
		cfg.LogPath = "/var/log/hybrid-failover-bot.log"
	}
	if cfg.AuditPath == "" {
		cfg.AuditPath = "/var/log/hybrid-failover-bot.audit.log"
	}
}

func legacyString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return strings.TrimSpace(s), ok && s != ""
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token is required")
	}
	if len(c.AdminIDs) == 0 {
		return errors.New("admin_ids is required")
	}
	switch c.Policy {
	case "outage-only", "prefer-primary", "fastest":
	default:
		return fmt.Errorf("unsupported policy %q", c.Policy)
	}
	for i, r := range c.Routers {
		if err := r.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (r RouterConfig) validate(index int) error {
	label := fmt.Sprintf("routers[%d]", index)
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%s: id is required", label)
	}
	if r.Local {
		return nil
	}
	if strings.TrimSpace(r.Host) == "" {
		return fmt.Errorf("%s (%s): host is required", label, r.ID)
	}
	if strings.TrimSpace(r.IdentityFile) == "" {
		return fmt.Errorf("%s (%s): identity_file is required for remote router", label, r.ID)
	}
	return nil
}
