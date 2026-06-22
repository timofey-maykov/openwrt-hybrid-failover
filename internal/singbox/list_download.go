package singbox

import "github.com/tmaykov/openwrt-hybrid-failover/internal/uci"

// ListDownloadSection reports whether community list downloads should use a proxy
// section and which UCI section name to route through.
func ListDownloadSection(pkg *uci.Package) (enabled bool, section string) {
	if pkg == nil {
		return false, ""
	}
	settings := pkg.Section("settings")
	if settings == nil || !settings.GetBool("download_lists_via_proxy", false) {
		return false, ""
	}
	section = settings.Get("download_lists_via_proxy_section", "")
	if section == "" {
		section = settings.Get("main_section", "")
	}
	return true, section
}
