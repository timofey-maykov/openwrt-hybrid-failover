package validation

import (
	"fmt"
	"regexp"
	"strings"
)

var uciKeyRe = regexp.MustCompile(`^podkop\.[a-zA-Z0-9_@-]+(\[[0-9]+\])?\.[a-zA-Z0-9_-]+$`)
var uciSectionRe = regexp.MustCompile(`^podkop\.[a-zA-Z0-9_@-]+(\[[0-9]+\])?$`)

func ValidateUCIKey(key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return fmt.Errorf("uci key is empty")
	}
	if !uciKeyRe.MatchString(trimmed) {
		return fmt.Errorf("unsupported uci key format: %q", key)
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
		return fmt.Errorf("unsupported uci section format: %q", key)
	}
	return nil
}
