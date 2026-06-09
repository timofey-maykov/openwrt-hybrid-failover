package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileAndEnvOverride(t *testing.T) {
	t.Setenv("HF_BOT_TOKEN", "env-token")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bot.json")
	data := `{
		"token": "file-token",
		"admin_ids": [1001, 1002],
		"log_path": "/tmp/hybrid-failover-bot.log",
		"audit_path": "/tmp/hybrid-failover-bot.audit.log",
		"clash_api": "http://127.0.0.1:9090",
		"routing_init_script": "/etc/init.d/legacy-routing",
		"policy": "outage-only",
		"probe_timeout_seconds": 5
	}`
	if err := os.WriteFile(cfgPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Token != "env-token" {
		t.Fatalf("expected env token override, got %q", cfg.Token)
	}
	if len(cfg.AdminIDs) != 2 {
		t.Fatalf("unexpected admin ids len: %d", len(cfg.AdminIDs))
	}
	if cfg.RoutingInitScript != "/etc/init.d/legacy-routing" {
		t.Fatalf("routing_init_script: got %q", cfg.RoutingInitScript)
	}
}

func TestValidateRejectsMissingToken(t *testing.T) {
	cfg := Config{
		AdminIDs: []int64{1001},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestValidateRejectsUnknownPolicy(t *testing.T) {
	cfg := Config{
		Token:    "x",
		AdminIDs: []int64{1001},
		Policy:   "invalid-policy",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown policy")
	}
}

func TestValidateAcceptsFastestPolicy(t *testing.T) {
	cfg := Config{
		Token:    "x",
		AdminIDs: []int64{1001},
		Policy:   "fastest",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("fastest policy: %v", err)
	}
}

func TestLoadMigratesLegacyPodkopFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bot.json")
	data := `{
		"token": "file-token",
		"admin_ids": [1001],
		"log_path": "/var/log/podkop-telegram-bot.log",
		"audit_path": "/var/log/podkop-telegram-bot.audit.log",
		"podkop_init_script": "/etc/init.d/podkop",
		"policy": "outage-only"
	}`
	if err := os.WriteFile(cfgPath, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.RoutingInitScript != "/etc/init.d/hybrid-failover" {
		t.Fatalf("routing_init_script: got %q", cfg.RoutingInitScript)
	}
	if cfg.LogPath != "/var/log/hybrid-failover-bot.log" {
		t.Fatalf("log_path: got %q", cfg.LogPath)
	}
}
