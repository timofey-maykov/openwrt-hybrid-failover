package amnezia

import (
	"fmt"
	"net/url"
	"strings"
)

// DescribeLink returns a redacted human-readable summary of a proxy/vpn URI.
func DescribeLink(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty uri")
	}
	if strings.HasPrefix(raw, "vpn://") {
		decoded, err := DecodeVPNURI(raw)
		if err != nil {
			return "", err
		}
		return DescribeLink(decoded)
	}
	if strings.HasPrefix(raw, "awg2://") {
		return fmt.Sprintf("awg2 interface %s", AWG2InterfaceName("preview")), nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" {
		return u.Scheme + " (no host)", nil
	}
	if port != "" {
		return fmt.Sprintf("%s://%s:%s", u.Scheme, host, port), nil
	}
	return fmt.Sprintf("%s://%s", u.Scheme, host), nil
}
