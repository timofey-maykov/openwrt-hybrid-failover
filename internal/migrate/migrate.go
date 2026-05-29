package migrate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const targetSchema = 1

// MigrationChange is a uci CLI command without the "uci" prefix.
type MigrationChange struct {
	Cmd string
}

// PlanMigration returns uci set/delete commands needed to reach targetSchema.
func PlanMigration(pkg *uci.Package) []MigrationChange {
	if pkg == nil {
		return nil
	}
	settings := pkg.Section("settings")
	schema := 0
	if settings != nil {
		fmt.Sscanf(settings.Get("config_schema_version", "0"), "%d", &schema)
	}
	if schema >= targetSchema {
		return nil
	}

	pkgName := paths.UCIPackage
	var changes []MigrationChange
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		if sec.Get("failover_vpn_enabled", "") == "" {
			changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.failover_vpn_enabled=0", pkgName, name)})
		}
		conn := sec.Get("connection_type", "")
		links := sec.GetList("failover_proxy_links")
		if conn == "vpn" && len(links) > 0 && !sec.GetBool("failover_vpn_enabled", false) {
			changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.failover_vpn_enabled=1", pkgName, name)})
		}
		if sec.Get("urltest_interrupt_exist_connections", "") == "" {
			changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.urltest_interrupt_exist_connections=0", pkgName, name)})
		}
	}
	if settings == nil || settings.Get("cache_path", "") == "" {
		changes = append(changes, MigrationChange{fmt.Sprintf("set %s.settings.cache_path=%s", pkgName, paths.SingboxCache)})
	}
	changes = append(changes, MigrationChange{fmt.Sprintf("set %s.settings.config_schema_version=%d", pkgName, targetSchema)})
	return changes
}

// Run applies schema migrations and imports legacy UCI once if needed.
func Run(configPath string, dryRun bool) (changed bool, err error) {
	if configPath == "" {
		configPath = paths.UCIConfig
	}
	imported, err := importLegacyUCI(configPath, dryRun)
	if err != nil {
		return false, err
	}

	pkg, err := uci.Load(configPath)
	if err != nil {
		return imported, err
	}
	changes := PlanMigration(pkg)
	if len(changes) == 0 && !imported {
		warnLegacyScripts()
		return false, nil
	}

	if dryRun {
		return len(changes) > 0 || imported, nil
	}
	for _, c := range changes {
		args := strings.Fields(c.Cmd)
		if err := exec.Command("uci", args...).Run(); err != nil {
			return false, fmt.Errorf("uci %s: %w", c.Cmd, err)
		}
	}
	if len(changes) > 0 {
		if err := exec.Command("uci", "commit", paths.UCIPackage).Run(); err != nil {
			return false, err
		}
	}
	disableLegacyInit(dryRun)
	warnLegacyScripts()
	return len(changes) > 0 || imported, nil
}

func disableLegacyInit(dryRun bool) {
	if _, err := os.Stat(paths.LegacyInitScript); err != nil {
		return
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "migrate: would disable %s\n", paths.LegacyInitScript)
		return
	}
	_ = exec.Command(paths.LegacyInitScript, "disable").Run()
}

func warnLegacyScripts() {
	if _, err := os.Stat(paths.LegacyRoutingBinary); err == nil {
		fmt.Fprintf(os.Stderr, "warning: conflicting routing binary %s; use hybrid-failover only\n", paths.LegacyRoutingBinary)
	}
	if _, err := os.Stat(paths.LegacyFailoverHook); err == nil {
		fmt.Fprintf(os.Stderr, "warning: legacy failover hook %s found; remove it to avoid double failover\n", paths.LegacyFailoverHook)
	}
}

func importLegacyUCI(dest string, dryRun bool) (bool, error) {
	if _, err := os.Stat(dest); err == nil {
		return false, nil
	}
	if _, err := os.Stat(paths.LegacyUCIConfig); err != nil {
		return false, nil
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "migrate: would import %s -> %s\n", paths.LegacyUCIConfig, dest)
		return true, nil
	}
	if err := os.MkdirAll("/etc/config", 0o755); err != nil {
		return false, err
	}
	src, err := os.Open(paths.LegacyUCIConfig)
	if err != nil {
		return false, err
	}
	defer src.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false, err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return false, err
	}
	fmt.Fprintf(os.Stderr, "migrated UCI: %s -> %s\n", paths.LegacyUCIConfig, dest)
	return true, nil
}
