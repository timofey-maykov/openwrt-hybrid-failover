package plan

import (
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const ModeNative = "native"
const ModeSingbox = "singbox"

func EngineMode(pkg *uci.Package) string {
	if pkg == nil {
		return ModeNative
	}
	settings := pkg.Section("settings")
	if settings == nil {
		return ModeNative
	}
	mode := strings.TrimSpace(settings.Get("engine_mode", ModeNative))
	if mode == ModeNative {
		return ModeNative
	}
	return ModeSingbox
}

func NativeEnabled(pkg *uci.Package) bool {
	return EngineMode(pkg) == ModeNative
}

func LoadEngineMode() string {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return ModeNative
	}
	return EngineMode(pkg)
}
