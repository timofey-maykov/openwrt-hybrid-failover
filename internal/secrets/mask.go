// Package secrets hides credentials in text that is about to be shown or
// logged: subscription links carry a personal token, proxy links carry keys.
package secrets

import (
	"regexp"
	"strings"
)

var secretURLRe = regexp.MustCompile(`(?i)\b(?:https?|vless|vmess|trojan|ss|ssr|hy2|hysteria2?|tuic|socks5?|wireguard|awg2|amneziawg)://[^\s'"<>]+`)

// MaskURL hides what makes a link a credential (user info, path, query) and
// keeps enough to recognise it: scheme, host and the last characters.
func MaskURL(u string) string {
	u = strings.TrimSpace(u)
	i := strings.Index(u, "://")
	if i < 0 {
		return u
	}
	scheme, rest := u[:i+3], u[i+3:]
	// In these schemes the "host" part is the whole encoded config: nothing in
	// it is safe to show.
	switch strings.ToLower(strings.TrimSuffix(scheme, "://")) {
	case "amneziawg", "vmess", "ssr":
		return scheme + "•••"
	}
	end := strings.IndexAny(rest, "/?#")
	authority, tail := rest, ""
	if end >= 0 {
		authority, tail = rest[:end], rest[end:]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = "•••@" + authority[at+1:]
	}
	tail = strings.TrimLeft(tail, "/?#")
	switch {
	case tail == "":
		return scheme + authority
	case len(tail) <= 4:
		return scheme + authority + "/…"
	default:
		return scheme + authority + "/…" + tail[len(tail)-4:]
	}
}

// MaskSecrets masks every proxy or subscription link found in free text.
func MaskSecrets(text string) string {
	return secretURLRe.ReplaceAllStringFunc(text, MaskURL)
}
