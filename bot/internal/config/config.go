package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Token               string  `json:"token"`
	AdminIDs            []int64 `json:"admin_ids"`
	LogPath             string  `json:"log_path"`
	AuditPath           string  `json:"audit_path"`
	ClashAPI            string  `json:"clash_api"`
	RoutingInitScript   string  `json:"routing_init_script"`
	Policy              string  `json:"policy"`
	ProbeTimeoutSeconds int     `json:"probe_timeout_seconds"`
}

func Load(path string) (Config, error) {
	var cfg Config
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	var legacy struct {
		PodkopInitScript string `json:"podkop_init_script"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	_ = json.Unmarshal(raw, &legacy)
	if cfg.RoutingInitScript == "" && legacy.PodkopInitScript != "" {
		cfg.RoutingInitScript = legacy.PodkopInitScript
	}
	if envToken := strings.TrimSpace(os.Getenv("PODKOP_BOT_TOKEN")); envToken != "" {
		cfg.Token = envToken
	}
	if cfg.Policy == "" {
		cfg.Policy = "outage-only"
	}
	if cfg.ProbeTimeoutSeconds <= 0 {
		cfg.ProbeTimeoutSeconds = 5
	}
	if cfg.ClashAPI == "" {
		cfg.ClashAPI = "http://127.0.0.1:9090"
	}
	if cfg.RoutingInitScript == "" {
		cfg.RoutingInitScript = "/etc/init.d/podkop"
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
