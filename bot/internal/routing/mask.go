package routing

import (
	"regexp"
	"strings"
)

var secretURLRe = regexp.MustCompile(`(?i)\b(?:https?|vless|vmess|trojan|ss|ssr|hy2|hysteria2?|tuic|socks5?|wireguard)://[^\s'"<>]+`)

// MaskURL hides what makes a link a credential (user info, path, query) and
// keeps enough to recognise it: scheme, host and the last characters.
func MaskURL(u string) string {
	u = strings.TrimSpace(u)
	i := strings.Index(u, "://")
	if i < 0 {
		return u
	}
	scheme, rest := u[:i+3], u[i+3:]
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

// MaskSecrets masks every proxy or subscription link found in free text, so
// command output and logs sent to a chat do not leak them.
func MaskSecrets(text string) string {
	return secretURLRe.ReplaceAllStringFunc(text, MaskURL)
}
