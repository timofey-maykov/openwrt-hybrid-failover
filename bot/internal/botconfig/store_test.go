package botconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetPendingValidateApplyAndRollback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bot.json")
	t.Setenv("HF_BOT_TOKEN", "x")
	initial := `{
  "token": "x",
  "admin_ids": [1],
  "policy": "outage-only",
  "clash_api": "http://127.0.0.1:9090",
  "routing_init_script": "/etc/init.d/hybrid-failover"
}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewStore(path)
	if err := store.SetPendingKey("policy", "prefer-primary"); err != nil {
		t.Fatalf("set pending: %v", err)
	}
	if err := store.ValidatePending(); err != nil {
		t.Fatalf("validate pending: %v", err)
	}
	if err := store.ApplyPending(); err != nil {
		t.Fatalf("apply pending: %v", err)
	}

	activeRaw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(activeRaw) == initial {
		t.Fatal("expected active config to change after apply")
	}

	if err := store.SetPendingKey("policy", "outage-only"); err != nil {
		t.Fatalf("set pending: %v", err)
	}
	if err := store.RollbackPending(); err != nil {
		t.Fatalf("rollback pending: %v", err)
	}
	if _, err := os.Stat(path + ".pending"); !os.IsNotExist(err) {
		t.Fatalf("expected pending file removed, got err=%v", err)
	}
}

func TestSetPendingEditableBotFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bot.json")
	initial := `{
  "token": "x",
  "admin_ids": [1],
  "policy": "outage-only",
  "clash_api": "http://127.0.0.1:9090",
  "routing_init_script": "/etc/init.d/hybrid-failover",
  "probe_timeout_seconds": 5
}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)

	if err := store.SetPendingKey("token", "new-token"); err != nil {
		t.Fatalf("set token: %v", err)
	}
	if err := store.SetPendingKey("admin_ids", "100, 200"); err != nil {
		t.Fatalf("set admin_ids: %v", err)
	}
	if err := store.SetPendingKey("probe_timeout_seconds", "7"); err != nil {
		t.Fatalf("set probe timeout: %v", err)
	}
	cfg, err := store.LoadPending()
	if err != nil {
		t.Fatalf("load pending: %v", err)
	}
	if cfg.Token != "new-token" || len(cfg.AdminIDs) != 2 || cfg.ProbeTimeoutSeconds != 7 {
		t.Fatalf("unexpected cfg after set: %+v", cfg)
	}
}
