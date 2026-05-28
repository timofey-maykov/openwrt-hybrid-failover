package telegram

import (
	"fmt"
	"strconv"
	"strings"
)

var paramAliases = map[string]string{
	"disable_quic":                         "podkop.settings.disable_quic",
	"cache_path":                           "podkop.settings.cache_path",
	"urltest_interval":                     "podkop.glob.urltest_interval",
	"urltest_tolerance":                    "podkop.glob.urltest_tolerance",
	"urltest_idle_timeout":                 "podkop.glob.urltest_idle_timeout",
	"urltest_interrupt_exist_connections":  "podkop.glob.urltest_interrupt_exist_connections",
	"policy":                               "podkop.glob.failover_policy",
}

func resolveParamKey(input string) string {
	key := strings.TrimSpace(input)
	if resolved, ok := paramAliases[key]; ok {
		return resolved
	}
	return key
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
