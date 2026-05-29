package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

type Config struct {
	Token                       string  `json:"token"`
	AdminIDs                    []int64 `json:"admin_ids"`
	ViewerIDs                   []int64 `json:"viewer_ids"`
	LogPath                     string  `json:"log_path"`
	AuditPath                   string  `json:"audit_path"`
	ClashAPI                    string  `json:"clash_api"`
	RoutingInitScript           string  `json:"routing_init_script"`
	UCIPackage                  string  `json:"uci_package"`
	MainSection                 string  `json:"main_section"`
	Policy                      string  `json:"policy"`
	ProbeTimeoutSeconds         int     `json:"probe_timeout_seconds"`
	NotifyFailoverEnabled       bool    `json:"notify_failover_enabled"`
	NotifyFailoverIntervalSeconds int   `json:"notify_failover_interval_seconds"`
}

func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
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

func (c Config) Validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token is required")
	}
	if len(c.AdminIDs) == 0 {
		return errors.New("admin_ids is required")
	}
	switch c.Policy {
	case "outage-only", "prefer-primary":
	default:
		return fmt.Errorf("unsupported policy %q", c.Policy)
	}
	return nil
}
