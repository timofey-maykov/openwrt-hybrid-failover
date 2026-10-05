package subscription

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const refreshCronMarker = "/usr/sbin/hybrid-failover subscription-refresh"

// InstallCron registers a crontab entry for periodic subscription refresh.
// It is off unless subscription_urls is set and subscription_update_interval is not off.
func InstallCron(uciPath string) error {
	RemoveCron()
	if uciPath == "" {
		return nil
	}
	pkg, err := uci.Load(uciPath)
	if err != nil {
		return err
	}
	settings := pkg.Section("settings")
	if settings == nil || len(settings.GetList("subscription_urls")) == 0 {
		return nil
	}
	interval := settings.Get("subscription_update_interval", "off")
	job, on, ok := cronJobForInterval(interval)
	if !ok {
		return fmt.Errorf("invalid subscription_update_interval %q", interval)
	}
	if !on {
		return nil
	}
	cmd := exec.Command("sh", "-c", fmt.Sprintf("(crontab -l 2>/dev/null; echo %q) | crontab -", job))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install cron: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveCron drops hybrid-failover subscription-refresh entries from crontab.
func RemoveCron() {
	_ = exec.Command("sh", "-c",
		`(crontab -l 2>/dev/null | grep -v "`+refreshCronMarker+`") | crontab -`,
	).Run()
}

// cronJobForInterval returns the crontab line. ok is false for an unknown value,
// on is false for "off". The minute differs from the list update (13) on purpose.
func cronJobForInterval(interval string) (job string, on, ok bool) {
	switch strings.TrimSpace(interval) {
	case "", "off", "0":
		return "", false, true
	case "1h":
		return "37 * * * * " + refreshCronMarker, true, true
	case "3h":
		return "37 */3 * * * " + refreshCronMarker, true, true
	case "6h":
		return "37 */6 * * * " + refreshCronMarker, true, true
	case "12h":
		return "37 */12 * * * " + refreshCronMarker, true, true
	case "1d":
		return "37 9 * * * " + refreshCronMarker, true, true
	default:
		return "", false, false
	}
}
