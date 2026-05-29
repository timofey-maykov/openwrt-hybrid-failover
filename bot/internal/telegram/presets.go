package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

func buildParamAliases(mainSection string) map[string]string {
	if mainSection == "" {
		mainSection = paths.DefaultMainSection
	}
	pkg := paths.UCIPackage
	return map[string]string{
		"disable_quic":                        pkg + ".settings.disable_quic",
		"cache_path":                          pkg + ".settings.cache_path",
		"urltest_interval":                    pkg + "." + mainSection + ".urltest_check_interval",
		"urltest_check_interval":              pkg + "." + mainSection + ".urltest_check_interval",
		"urltest_tolerance":                   pkg + "." + mainSection + ".urltest_tolerance",
		"urltest_idle_timeout":                pkg + "." + mainSection + ".urltest_idle_timeout",
		"urltest_interrupt_exist_connections": pkg + "." + mainSection + ".urltest_interrupt_exist_connections",
		"policy":                              pkg + "." + mainSection + ".failover_policy",
	}
}

func resolveParamKey(input, mainSection string) string {
	key := strings.TrimSpace(input)
	if resolved, ok := buildParamAliases(mainSection)[key]; ok {
		return resolved
	}
	return key
}

func uciSectionKey(pkg, section, option string) string {
	if pkg == "" {
		pkg = paths.UCIPackage
	}
	if section == "" {
		section = paths.DefaultMainSection
	}
	return pkg + "." + section + "." + option
}

func onOffToBoolValue(in string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "on", "1", "true", "yes":
		return "1", nil
	case "off", "0", "false", "no":
		return "0", nil
	default:
		return "", fmt.Errorf("ожидается on/off")
	}
}

func parsePositiveInt(value string) (string, error) {
	v, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || v <= 0 {
		return "", fmt.Errorf("ожидается положительное число")
	}
	return strconv.Itoa(v), nil
}

func parseDurationSeconds(value string) (string, error) {
	return validation.NormalizeSingboxDuration(value)
}
