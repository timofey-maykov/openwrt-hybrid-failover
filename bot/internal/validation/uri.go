package validation

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

func ValidateProxyURI(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errors.New("uri is empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("parse uri: %w", err)
	}
	switch parsed.Scheme {
	case "vless", "trojan", "ss", "vpn":
		return nil
	default:
		return fmt.Errorf("unsupported uri scheme %q", parsed.Scheme)
	}
}
