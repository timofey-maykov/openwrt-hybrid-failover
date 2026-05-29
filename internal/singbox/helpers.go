package singbox

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
)

func parseIntDefault(s string, def int) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	if n == 0 {
		return def
	}
	return n
}

func isIPv4(s string) bool {
	return net.ParseIP(s) != nil && strings.Contains(s, ".")
}

func urlHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		return strings.Split(raw, "/")[0]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return strings.Split(raw, "/")[0]
	}
	if u.Hostname() != "" {
		return u.Hostname()
	}
	return strings.Split(raw, "://")[1]
}

func urlPort(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	if !strings.Contains(raw, "://") {
		if h, p, err := net.SplitHostPort(raw); err == nil && p != "" {
			if n, err := strconv.Atoi(p); err == nil {
				return n
			}
			_ = h
		}
		return def
	}
	u, err := url.Parse(raw)
	if err != nil {
		return def
	}
	if u.Port() != "" {
		if n, err := strconv.Atoi(u.Port()); err == nil {
			return n
		}
	}
	return def
}

func urlPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Path == "" || u.Path == "/" {
		return "/dns-query"
	}
	return u.Path
}

func fileExtension(u string) string {
	base := path.Base(strings.Split(u, "?")[0])
	if i := strings.LastIndex(base, "."); i >= 0 {
		return strings.ToLower(base[i+1:])
	}
	return ""
}

func fileBaseName(u string) string {
	base := path.Base(strings.Split(u, "?")[0])
	if i := strings.LastIndex(base, "."); i >= 0 {
		return base[:i]
	}
	return base
}

func appendUniqueStrings(existing []string, add ...string) []string {
	seen := make(map[string]struct{}, len(existing))
	out := append([]string(nil), existing...)
	for _, s := range existing {
		seen[s] = struct{}{}
	}
	for _, s := range add {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func rulesetFormat(ext string) string {
	switch ext {
	case "json":
		return "source"
	case "srs":
		return "binary"
	default:
		return "source"
	}
}
