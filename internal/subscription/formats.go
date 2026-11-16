package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

func parseSubscription(body []byte) ([]string, error) {
	text := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	if !strings.Contains(text, "://") {
		compact := strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) {
				return -1
			}
			return r
		}, text)
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			d, err := enc.DecodeString(compact)
			decoded := strings.TrimSpace(string(d))
			if err == nil && utf8.Valid(d) && (strings.Contains(decoded, "://") || strings.HasPrefix(decoded, "{") || strings.HasPrefix(decoded, "[")) {
				text = decoded
				break
			}
		}
	}
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		var value any
		if json.Unmarshal([]byte(text), &value) != nil {
			return nil, fmt.Errorf("subscription contains invalid JSON")
		}
		return collectJSON(value, "", 0), nil
	}
	if strings.HasPrefix(text, "proxies:") || strings.Contains(text, "\nproxies:") {
		var doc struct {
			Proxies []map[string]any `yaml:"proxies"`
		}
		if yaml.Unmarshal([]byte(text), &doc) != nil {
			return nil, fmt.Errorf("subscription contains invalid Clash YAML")
		}
		var entries []string
		for _, proxy := range doc.Proxies {
			if uri, ok := clashURI(proxy); ok {
				entries = append(entries, uri)
			}
		}
		return entries, nil
	}
	var entries []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			entries = append(entries, line)
		}
	}
	return entries, nil
}

func collectJSON(value any, name string, depth int) []string {
	if depth > 8 {
		return nil
	}
	var entries []string
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			entries = append(entries, collectJSON(item, name, depth+1)...)
		}
	case map[string]any:
		if title := textValue(v, "remarks"); title != "" {
			name = title
		}
		if proxies, ok := v["proxies"].([]any); ok {
			for _, proxy := range proxies {
				if u, ok := clashURI(object(proxy)); ok {
					entries = append(entries, u)
				}
			}
			return entries
		}
		if out, ok := v["outbounds"]; ok {
			return collectJSON(out, name, depth+1)
		}
		if textValue(v, "protocol") != "" {
			return xrayURIs(v, name)
		}
		if name == "" {
			name = textValue(v, "tag")
		}
		if uri, ok := singboxURI(v, name); ok {
			entries = append(entries, uri)
		}
	}
	return entries
}

func object(v any) map[string]any                   { m, _ := v.(map[string]any); return m }
func textValue(m map[string]any, key string) string { s, _ := m[key].(string); return s }
func enabled(v any) bool                            { b, _ := v.(bool); return b }
func number(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		if n == float64(int(n)) {
			return int(n)
		}
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}
func listText(v any) string {
	switch a := v.(type) {
	case string:
		return a
	case []any:
		var out []string
		for _, x := range a {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return strings.Join(out, ",")
	}
	return ""
}
func put(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func singboxURI(m map[string]any, name string) (string, bool) {
	kind := textValue(m, "type")
	switch kind {
	case "vless", "trojan", "shadowsocks", "socks", "hysteria2":
	default:
		return "", false
	}
	if textValue(m, "detour") != "" {
		return "", false
	}
	server := textValue(m, "server")
	port := number(m["server_port"])
	if server == "" || port <= 0 || port > 65535 {
		return "", false
	}
	q := make(url.Values)
	if fm, present := m["finalmask"]; present {
		if text, ok := fm.(string); ok {
			q.Set("fm", text)
		} else {
			raw, err := json.Marshal(fm)
			if err != nil {
				return "", false
			}
			q.Set("fm", string(raw))
		}
	}
	tls := object(m["tls"])
	transportKind := textValue(object(m["transport"]), "type")
	if (kind == "shadowsocks" || kind == "socks") && (enabled(tls["enabled"]) || (transportKind != "" && transportKind != "tcp")) {
		return "", false
	}
	if !tlsQuery(q, tls) {
		return "", false
	}
	if !transportQuery(q, object(m["transport"])) {
		return "", false
	}
	u := url.URL{Host: net.JoinHostPort(server, strconv.Itoa(port)), Fragment: name}
	switch kind {
	case "vless":
		if textValue(m, "uuid") == "" || (textValue(m, "encryption") != "" && textValue(m, "encryption") != "none") {
			return "", false
		}
		u.Scheme = "vless"
		u.User = url.User(textValue(m, "uuid"))
		put(q, "flow", textValue(m, "flow"))
		put(q, "packetEncoding", textValue(m, "packet_encoding"))
	case "trojan":
		if textValue(m, "password") == "" {
			return "", false
		}
		u.Scheme = "trojan"
		u.User = url.User(textValue(m, "password"))
		if q.Get("security") == "" {
			q.Set("security", "none")
		}
	case "shadowsocks":
		if textValue(m, "method") == "" || textValue(m, "password") == "" || textValue(m, "plugin") != "" {
			return "", false
		}
		u.Scheme = "ss"
		u.User = url.User(base64.RawURLEncoding.EncodeToString([]byte(textValue(m, "method") + ":" + textValue(m, "password"))))
	case "socks":
		version := textValue(m, "version")
		if version == "" {
			version = "5"
		}
		if version != "5" && version != "4" && version != "4a" {
			return "", false
		}
		u.Scheme = "socks" + version
		if username := textValue(m, "username"); username != "" {
			u.User = url.UserPassword(username, textValue(m, "password"))
		}
	case "hysteria2":
		if enabled(object(tls["reality"])["enabled"]) {
			return "", false
		}
		if textValue(m, "password") == "" {
			return "", false
		}
		u.Scheme = "hy2"
		u.User = url.User(textValue(m, "password"))
		obfs := object(m["obfs"])
		if typ := textValue(obfs, "type"); typ != "" {
			if typ != "salamander" {
				return "", false
			}
			q.Set("obfs", typ)
			put(q, "obfs-password", textValue(obfs, "password"))
		}
		if n := number(m["up_mbps"]); n > 0 {
			q.Set("upmbps", strconv.Itoa(n))
		}
		if n := number(m["down_mbps"]); n > 0 {
			q.Set("downmbps", strconv.Itoa(n))
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), true
}

func tlsQuery(q url.Values, tls map[string]any) bool {
	if !enabled(tls["enabled"]) {
		if _, present := tls["pinned_peer_cert_sha256"]; present {
			return false
		}
		return true
	}
	// Certificate pin formats are not interchangeable. A sing-box SPKI pin must
	// never be converted into Hysteria's leaf-certificate pinSHA256 parameter.
	for _, key := range []string{"certificate", "certificate_path", "certificate_public_key_sha256", "client_certificate", "client_key", "client_certificate_path", "client_key_path", "min_version", "max_version", "cipher_suites", "disable_sni"} {
		if specified(tls[key]) {
			return false
		}
	}
	if enabled(object(tls["ech"])["enabled"]) {
		return false
	}
	q.Set("security", "tls")
	put(q, "sni", textValue(tls, "server_name"))
	put(q, "alpn", listText(tls["alpn"]))
	if value, present := tls["pinned_peer_cert_sha256"]; present {
		text, ok := value.(string)
		if !ok {
			return false
		}
		q.Set("pcs", text)
	}
	if enabled(tls["insecure"]) {
		q.Set("insecure", "1")
	}
	utls := object(tls["utls"])
	if enabled(utls["enabled"]) {
		put(q, "fp", textValue(utls, "fingerprint"))
	}
	reality := object(tls["reality"])
	if enabled(reality["enabled"]) {
		q.Set("security", "reality")
		put(q, "pbk", textValue(reality, "public_key"))
		put(q, "sid", textValue(reality, "short_id"))
	}
	return true
}

func transportQuery(q url.Values, tr map[string]any) bool {
	kind := textValue(tr, "type")
	switch kind {
	case "", "tcp":
		return true
	case "ws", "httpupgrade", "xhttp", "splithttp":
		// The share URI carries Host but not arbitrary transport headers.
		for key, value := range object(tr["headers"]) {
			if !strings.EqualFold(key, "Host") {
				return false
			}
			put(q, "host", fmt.Sprint(value))
		}
		if number(tr["max_early_data"]) > 0 || textValue(tr, "early_data_header_name") != "" {
			return false
		}
		if kind == "splithttp" {
			kind = "xhttp"
		}
		if kind == "xhttp" {
			mode := textValue(tr, "mode")
			if mode != "" && mode != "auto" && mode != "stream-one" {
				return false
			}
			put(q, "mode", mode)
		}
		q.Set("type", kind)
		put(q, "path", textValue(tr, "path"))
	case "grpc":
		q.Set("type", "grpc")
		put(q, "serviceName", textValue(tr, "service_name"))
		put(q, "authority", textValue(tr, "authority"))
		put(q, "idle_timeout", textValue(tr, "idle_timeout"))
		put(q, "ping_timeout", textValue(tr, "ping_timeout"))
		if enabled(tr["permit_without_stream"]) {
			q.Set("permit_without_stream", "1")
		}
	default:
		return false
	}
	return true
}

func specified(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	case bool:
		return x
	default:
		return true
	}
}
