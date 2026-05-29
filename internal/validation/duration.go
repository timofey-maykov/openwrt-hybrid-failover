package validation

import (
	"fmt"
	"strconv"
	"strings"
)

// Sing-box duration options in UCI (need unit: 60s, not 60).
var singboxDurationOptionSuffixes = []string{
	"urltest_idle_timeout",
	"urltest_check_interval",
	"urltest_interval", // legacy alias still seen in UCI
}

func IsSingboxDurationUCIKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	dot := strings.LastIndex(key, ".")
	option := key
	if dot >= 0 {
		option = key[dot+1:]
	}
	for _, suffix := range singboxDurationOptionSuffixes {
		if option == suffix {
			return true
		}
	}
	return false
}

// NormalizeSingboxDuration ensures a time unit for sing-box (appends "s" to plain seconds).
func NormalizeSingboxDuration(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("ожидается длительность (например 60s или 30)")
	}
	if strings.ContainsAny(trimmed, "smhdSMHD") {
		return trimmed, nil
	}
	v, err := strconv.Atoi(trimmed)
	if err != nil || v <= 0 {
		return "", fmt.Errorf("ожидается положительное число секунд или длительность с единицей (60s, 5m)")
	}
	return strconv.Itoa(v) + "s", nil
}

func NormalizeUCIOptionValue(key, value string) (string, error) {
	value = NormalizeValue(value)
	if value == "" {
		return "", nil
	}
	if IsSingboxDurationUCIKey(key) {
		return NormalizeSingboxDuration(value)
	}
	return value, nil
}
