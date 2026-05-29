package singbox

import (
	"fmt"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func (b *Builder) configureExperimental() {
	b.cfg.Experimental = map[string]any{
		"clash_api": map[string]any{
			"external_controller": b.clashAPIListen(),
		},
		"cache_file": map[string]any{
			"enabled":     true,
			"path":        b.cachePath(),
			"store_fakeip": true,
		},
	}

	settings := b.pkg.Section("settings")
	if settings == nil || !settings.GetBool("enable_yacd", false) {
		return
	}
	clash := b.cfg.Experimental["clash_api"].(map[string]any)
	clash["external_ui"] = "ui"
	if secret := settings.Get("yacd_secret_key", ""); secret != "" {
		clash["secret"] = secret
	}
}

func (b *Builder) clashAPIListen() string {
	settings := b.pkg.Section("settings")
	if settings != nil {
		if v := settings.Get("clash_api_listen", ""); v != "" {
			return v
		}
		if settings.GetBool("enable_yacd", false) && settings.GetBool("enable_yacd_wan_access", false) {
			return fmt.Sprintf("0.0.0.0:%d", ClashAPIPort)
		}
		if addr := settings.Get("service_listen_address", ""); addr != "" {
			return formatListenAddr(addr)
		}
	}
	if lan := detectLANAddress(); lan != "" {
		return formatListenAddr(lan)
	}
	return DefaultClashListen
}

func (b *Builder) cachePath() string {
	if settings := b.pkg.Section("settings"); settings != nil {
		if cp := settings.Get("cache_path", ""); cp != "" {
			return cp
		}
	}
	return DefaultCachePath
}

func (b *Builder) outputNetworkInterface() string {
	settings := b.pkg.Section("settings")
	if settings == nil {
		return ""
	}
	return settings.Get("output_network_interface", "")
}

func formatListenAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return DefaultClashListen
	}
	if strings.Contains(addr, ":") {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, ClashAPIPort)
}

func detectLANAddress() string {
	out, err := uci.Exec("get", "network.lan.ipaddr")
	if err != nil {
		return ""
	}
	out = strings.TrimSpace(strings.Split(out, "/")[0])
	if out == "" {
		return ""
	}
	return out
}
