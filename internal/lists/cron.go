package lists

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const listUpdateCronMarker = "/usr/sbin/hybrid-failover list-update"

// InstallCron registers a crontab entry for periodic list updates when lists are configured.
func InstallCron(uciPath string) error {
	if uciPath == "" {
		return nil
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return err
	}
	if !packageHasLists(pkg) {
		RemoveCron()
		return nil
	}
	settings := pkg.Section("settings")
	interval := "1d"
	if settings != nil {
		if v := settings.Get("update_interval", ""); v != "" {
			interval = v
		}
	}
	job, ok := cronJobForInterval(interval)
	if !ok {
		return fmt.Errorf("invalid update_interval %q", interval)
	}
	RemoveCron()
	cmd := exec.Command("sh", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo %q) | crontab -", job))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install cron: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveCron drops hybrid-failover list-update entries from crontab.
func RemoveCron() {
	_ = exec.Command("sh", "-c",
		`(crontab -l 2>/dev/null | grep -v "`+listUpdateCronMarker+`") | crontab -`,
	).Run()
}

func cronJobForInterval(interval string) (string, bool) {
	switch interval {
	case "1h":
		return "13 * * * * " + listUpdateCronMarker, true
	case "3h":
		return "13 */3 * * * " + listUpdateCronMarker, true
	case "12h":
		return "13 */12 * * * " + listUpdateCronMarker, true
	case "1d":
		return "13 9 * * * " + listUpdateCronMarker, true
	case "3d":
		return "13 9 */3 * * " + listUpdateCronMarker, true
	default:
		return "", false
	}
}

func packageHasLists(pkg *uci.Package) bool {
	if pkg == nil {
		return false
	}
	for _, name := range pkg.SectionNames("section") {
		if singbox.SectionHasEnabledLists(pkg.Section(name)) {
			return true
		}
	}
	return false
}
