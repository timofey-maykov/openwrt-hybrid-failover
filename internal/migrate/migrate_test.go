package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestPlanMigrationSchemaV1(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "hybrid-failover")
	content := `
config settings 'settings'

config section 'glob'
	option connection_type 'vpn'
	list failover_proxy_links 'vless://example'
`
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := uci.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	changes := PlanMigration(pkg)
	if len(changes) == 0 {
		t.Fatal("expected migration changes")
	}
	joined := strings.Join(func() []string {
		out := make([]string, len(changes))
		for i, c := range changes {
			out[i] = c.Cmd
		}
		return out
	}(), "\n")
	for _, want := range []string{
		"failover_vpn_enabled=1",
		"urltest_interrupt_exist_connections=0",
		"cache_path=" + paths.SingboxCache,
		"config_schema_version=1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in changes:\n%s", want, joined)
		}
	}
}

func TestPlanMigrationAlreadyAtSchema(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "hybrid-failover")
	content := `
config settings 'settings'
	option config_schema_version '1'
	option cache_path '/etc/sing-box/cache.db'
`
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg, err := uci.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if changes := PlanMigration(pkg); len(changes) != 0 {
		t.Fatalf("expected no changes, got %v", changes)
	}
}
