package uri

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ProxyOutbound struct {
	Type   string
	Tag    string
	Fields map[string]any
}

func ParseProxy(raw, tag string, udpOverTCP bool) (ProxyOutbound, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ProxyOutbound{}, fmt.Errorf("empty uri")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ProxyOutbound{}, fmt.Errorf("parse uri: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "vless":
		return parseVLESS(u, tag)
	case "trojan":
		return parseTrojan(u, tag)
	case "ss":
		return parseShadowsocks(u, tag, udpOverTCP)
	case "socks4", "socks4a", "socks5":
		return parseSocks(u, tag, scheme, udpOverTCP)
	case "hysteria2", "hy2":
		return parseHysteria2(u, tag)
	default:
		return ProxyOutbound{}, fmt.Errorf("unsupported scheme %q", scheme)
	}
}

func parseVLESS(u *url.URL, tag string) (ProxyOutbound, error) {
	port := u.Port()
	if port == "" {
		port = "443"
	}
	p, _ := strconv.Atoi(port)
	ob := map[string]any{
		"type":        "vless",
		"tag":         tag,
		"server":      u.Hostname(),
		"server_port": p,
		"uuid":        u.User.Username(),
	}
	q := u.Query()
	if flow := q.Get("flow"); flow != "" {
		ob["flow"] = flow
	}
	if pe := q.Get("packetEncoding"); pe != "" {
		ob["packet_encoding"] = pe
	}
	addTLSAndTransport(ob, q)
	return ProxyOutbound{Type: "vless", Tag: tag, Fields: ob}, nil
}

func parseTrojan(u *url.URL, tag string) (ProxyOutbound, error) {
	port := u.Port()
	if port == "" {
		port = "443"
	}
	p, _ := strconv.Atoi(port)
	ob := map[string]any{
		"type":        "trojan",
		"tag":         tag,
		"server":      u.Hostname(),
		"server_port": p,
		"password":    u.User.Username(),
	}
	addTLSAndTransport(ob, u.Query())
	return ProxyOutbound{Type: "trojan", Tag: tag, Fields: ob}, nil
}

func parseShadowsocks(u *url.URL, tag string, udpOverTCP bool) (ProxyOutbound, error) {
	port := u.Port()
	if port == "" {
		port = "8388"
	}
	p, _ := strconv.Atoi(port)
	userinfo := u.User.String()
	if !strings.Contains(userinfo, ":") {
		dec, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(userinfo, "="))
		if err != nil {
			dec, err = base64.StdEncoding.DecodeString(userinfo)
		}
		if err != nil {
			return ProxyOutbound{}, fmt.Errorf("decode ss userinfo: %w", err)
		}
		userinfo = string(dec)
	}
	parts := strings.SplitN(userinfo, ":", 2)
	if len(parts) != 2 {
		return ProxyOutbound{}, fmt.Errorf("invalid shadowsocks credentials")
	}
	ob := map[string]any{
		"type":        "shadowsocks",
		"tag":         tag,
		"server":      u.Hostname(),
		"server_port": p,
		"method":      parts[0],
		"password":    parts[1],
	}
	if udpOverTCP {
		ob["udp_over_tcp"] = map[string]any{"enabled": true, "version": 2}
	}
	return ProxyOutbound{Type: "shadowsocks", Tag: tag, Fields: ob}, nil
}

func parseSocks(u *url.URL, tag, scheme string, udpOverTCP bool) (ProxyOutbound, error) {
	port := u.Port()
	if port == "" {
		port = "1080"
	}
	p, _ := strconv.Atoi(port)
	version := strings.TrimPrefix(scheme, "socks")
	ob := map[string]any{
		"type":        "socks",
		"tag":         tag,
		"server":      u.Hostname(),
		"server_port": p,
		"version":     version,
	}
	if u.User != nil {
		if pass, ok := u.User.Password(); ok {
			ob["username"] = u.User.Username()
			ob["password"] = pass
		}
	}
	if udpOverTCP {
		ob["udp_over_tcp"] = map[string]any{"enabled": true, "version": 2}
	}
	return ProxyOutbound{Type: "socks", Tag: tag, Fields: ob}, nil
}

func parseHysteria2(u *url.URL, tag string) (ProxyOutbound, error) {
	host := u.Hostname()
	if host == "" {
		return ProxyOutbound{}, fmt.Errorf("hysteria2: missing host")
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	p, err := strconv.Atoi(port)
	if err != nil || p <= 0 {
		return ProxyOutbound{}, fmt.Errorf("hysteria2: invalid port %q", port)
	}
	password := hysteria2Password(u)
	if password == "" {
		return ProxyOutbound{}, fmt.Errorf("hysteria2: missing password")
	}

	q := u.Query()
	ob := map[string]any{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      host,
		"server_port": p,
		"password":    password,
		"tls":         hysteria2TLS(host, q),
	}
	if obfsType := firstNonEmpty(q.Get("obfs"), q.Get("obfs-type")); obfsType != "" {
		obfsPass := firstNonEmpty(q.Get("obfs-password"), q.Get("obfsPassword"))
		ob["obfs"] = map[string]any{
			"type":     obfsType,
			"password": obfsPass,
		}
	}
	for _, pair := range []struct{ uriKey, field string }{
		{"upmbps", "up_mbps"},
		{"downmbps", "down_mbps"},
		{"up", "up_mbps"},
		{"down", "down_mbps"},
	} {
		if v := q.Get(pair.uriKey); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				ob[pair.field] = n
			}
		}
	}
	return ProxyOutbound{Type: "hysteria2", Tag: tag, Fields: ob}, nil
}

func hysteria2Password(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	user := u.User.Username()
	if pass, ok := u.User.Password(); ok && pass != "" {
		// Official Hysteria userpass auth: sing-box expects "user:pass" as password.
		return user + ":" + pass
	}
	return user
}

func hysteria2TLS(host string, q url.Values) map[string]any {
	tls := map[string]any{
		"enabled": true,
	}
	if sni := firstNonEmpty(q.Get("sni"), q.Get("peer")); sni != "" {
		tls["server_name"] = sni
	} else {
		tls["server_name"] = host
	}
	if truthy(q.Get("insecure")) || truthy(q.Get("allowInsecure")) || truthy(q.Get("allow_insecure")) {
		tls["insecure"] = true
	}
	if pin := q.Get("pinSHA256"); pin != "" {
		tls["certificate_public_key_sha256"] = []string{pin}
	}
	if alpn := q.Get("alpn"); alpn != "" {
		tls["alpn"] = strings.Split(alpn, ",")
	}
	return tls
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func addTLSAndTransport(ob map[string]any, q url.Values) {
	sec := q.Get("security")
	if sec == "" {
		sec = "none"
	}
	netType := q.Get("type")
	if netType == "" {
		netType = "tcp"
	}
	if sec == "reality" {
		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": q.Get("sni"),
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": firstNonEmpty(q.Get("fp"), "chrome"),
			},
			"reality": map[string]any{
				"enabled":    true,
				"public_key": q.Get("pbk"),
				"short_id":   q.Get("sid"),
			},
		}
	} else if sec == "tls" {
		ob["tls"] = map[string]any{
			"enabled":     true,
			"server_name": q.Get("sni"),
		}
		if fp := q.Get("fp"); fp != "" {
			ob["tls"].(map[string]any)["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
	}
	switch netType {
	case "ws":
		ob["transport"] = map[string]any{
			"type": "ws",
			"path": q.Get("path"),
			"headers": map[string]string{
				"Host": q.Get("host"),
			},
		}
	case "grpc":
		ob["transport"] = map[string]any{
			"type":                       "grpc",
			"service_name":               q.Get("serviceName"),
			"idle_timeout":               q.Get("idle_timeout"),
			"ping_timeout":               q.Get("ping_timeout"),
			"permit_without_stream":      q.Get("permit_without_stream") == "1",
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
