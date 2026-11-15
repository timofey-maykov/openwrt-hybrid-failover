package subscription

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeURLs ignores empty DynamicList rows and repeated feeds without
// modifying any access token or query parameter in a subscription address.
func NormalizeURLs(urls []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	return out
}

func validateSubscriptionURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("subscription URL must be a complete http:// or https:// address")
	}
	return nil
}

// Some panels return fake VLESS nodes to display a compatibility notice in a
// VPN client's server list. Do not install those notices as working channels.
func providerNotice(entries []string) string {
	notice := ""
	count := 0
	for _, entry := range entries {
		u, err := url.Parse(entry)
		if err != nil || u.Scheme != "vless" || u.User == nil || u.User.Username() != "00000000-0000-0000-0000-000000000000" {
			return ""
		}
		count++
		name := entryName(entry)
		lower := strings.ToLower(name)
		if strings.Contains(lower, "приложение не поддерживается") || strings.Contains(lower, "unsupported client") || strings.Contains(lower, "unsupported application") {
			notice = name
		}
	}
	if count == 0 {
		return ""
	}
	return notice
}
