package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

var (
	uciKeyRe     = regexp.MustCompile(`^` + regexp.QuoteMeta(paths.UCIPackage) + `\.[a-zA-Z0-9_@-]+(\[[0-9]+\])?\.[a-zA-Z0-9_-]+$`)
	uciSectionRe = regexp.MustCompile(`^` + regexp.QuoteMeta(paths.UCIPackage) + `\.[a-zA-Z0-9_@-]+(\[[0-9]+\])?$`)
)

func ValidateUCIKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return fmt.Errorf("uci key is empty")
	}
	if !uciKeyRe.MatchString(trimmed) {
		return fmt.Errorf("unsupported uci key format: %q (expected prefix %s.)", key, paths.UCIPackage)
	}
	return nil
}

func NormalizeValue(input string) string {
	return strings.TrimSpace(input)
}

func ValidateUCISectionKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return fmt.Errorf("uci section is empty")
	}
	if !uciSectionRe.MatchString(trimmed) {
		return fmt.Errorf("unsupported uci section format: %q (expected prefix %s.)", key, paths.UCIPackage)
	}
	return nil
}
