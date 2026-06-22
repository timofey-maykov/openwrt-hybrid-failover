package migrate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clientrules"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const targetSchema = 4

// MigrationChange is a uci CLI command without the "uci" prefix.
type MigrationChange struct {
	Cmd string
}

func currentSchema(pkg *uci.Package) int {
	if pkg == nil {
		return 0
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return 0
	}
	schema := 0
	fmt.Sscanf(settings.Get("config_schema_version", "0"), "%d", &schema)
	return schema
}

// PlanMigration returns uci set/delete commands needed to reach targetSchema.
func PlanMigration(pkg *uci.Package) []MigrationChange {
	if pkg == nil {
		return nil
	}
	schema := currentSchema(pkg)
	if schema >= targetSchema {
		return nil
	}

	pkgName := paths.UCIPackage
	var changes []MigrationChange
	if schema < 1 {
		changes = append(changes, planSchemaV1(pkg, pkgName)...)
	}
	if schema < 2 {
		changes = append(changes, planSchemaV2ClientRules(pkg, pkgName)...)
	}
	if schema < 3 {
		changes = append(changes, planSchemaV3EngineMode(pkg, pkgName)...)
	}
	if schema < 4 {
		changes = append(changes, planSchemaV4NativeEngine(pkg, pkgName)...)
	}
	changes = append(changes, MigrationChange{fmt.Sprintf("set %s.settings.config_schema_version=%d", pkgName, targetSchema)})
	return changes
}

func planSchemaV1(pkg *uci.Package, pkgName string) []MigrationChange {
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
	settings := pkg.Section("settings")
	if settings == nil || settings.Get("cache_path", "") == "" {
		changes = append(changes, MigrationChange{fmt.Sprintf("set %s.settings.cache_path=%s", pkgName, paths.SingboxCache)})
	}
	return changes
}

func planSchemaV2ClientRules(pkg *uci.Package, pkgName string) []MigrationChange {
	if len(pkg.SectionNames("client_rule")) > 0 {
		return nil
	}
	legacy := clientrules.LegacyRulesFromPackage(pkg)
	if len(legacy) == 0 {
		return nil
	}
	var changes []MigrationChange
	for i, r := range legacy {
		name := fmt.Sprintf("migrated_%d", i)
		changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s=client_rule", pkgName, name)})
		changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.ip=%s", pkgName, name, r.IP)})
		changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.mode=%s", pkgName, name, r.Mode)})
		if r.Section != "" {
			changes = append(changes, MigrationChange{fmt.Sprintf("set %s.%s.section=%s", pkgName, name, r.Section)})
		}
	}
	return changes
}

func planSchemaV3EngineMode(pkg *uci.Package, pkgName string) []MigrationChange {
	settings := pkg.Section("settings")
	if settings == nil {
		return []MigrationChange{
			MigrationChange{fmt.Sprintf("set %s.settings.engine_mode=native", pkgName)},
		}
	}
	if settings.Get("engine_mode", "") != "" {
		return nil
	}
	return []MigrationChange{
		MigrationChange{fmt.Sprintf("set %s.settings.engine_mode=native", pkgName)},
	}
}

func planSchemaV4NativeEngine(pkg *uci.Package, pkgName string) []MigrationChange {
	settings := pkg.Section("settings")
	if settings == nil {
		return nil
	}
	if settings.Get("engine_mode", "") != "singbox" {
		return nil
	}
	return []MigrationChange{
		MigrationChange{fmt.Sprintf("set %s.settings.engine_mode=native", pkgName)},
	}
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
	removeLegacySingbox(dryRun)
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

// removeLegacySingbox stops the external sing-box service and removes the OpenWrt
// package when native engine is active. The in-tree engine no longer uses /usr/bin/sing-box.
func removeLegacySingbox(dryRun bool) {
	if _, err := os.Stat(paths.SingboxInit); os.IsNotExist(err) {
		if _, err := exec.LookPath("sing-box"); err != nil {
			return
		}
	} else if err != nil {
		return
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "migrate: would stop and remove sing-box package\n")
		return
	}
	if _, err := os.Stat(paths.SingboxInit); err == nil {
		_ = exec.Command(paths.SingboxInit, "stop").Run()
		_ = exec.Command(paths.SingboxInit, "disable").Run()
	}
	if _, err := exec.LookPath("opkg"); err == nil {
		out, err := exec.Command("opkg", "list-installed").CombinedOutput()
		if err == nil && strings.Contains(string(out), "sing-box") {
			if err := exec.Command("opkg", "remove", "--force-depends", "sing-box").Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: opkg remove sing-box: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "removed legacy sing-box package\n")
			}
		}
	}
	_ = os.Remove(paths.SingboxConfig)
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
