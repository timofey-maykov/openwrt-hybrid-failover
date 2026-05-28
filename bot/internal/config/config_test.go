package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromFileAndEnvOverride(t *testing.T) {
	t.Setenv("PODKOP_BOT_TOKEN", "env-token")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bot.json")
	data := `{
		"token": "file-token",
		"admin_ids": [1001, 1002],
		"log_path": "/tmp/podkop-telegram-bot.log",
		"audit_path": "/tmp/podkop-telegram-bot.audit.log",
		"clash_api": "http://127.0.0.1:9090",
		"podkop_init_script": "/etc/init.d/podkop",
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
		Policy:   "fastest",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unknown policy")
	}
}
