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
	port := u.Port()
	if port == "" {
		port = "443"
	}
	p, _ := strconv.Atoi(port)
	ob := map[string]any{
		"type":        "hysteria2",
		"tag":         tag,
		"server":      u.Hostname(),
		"server_port": p,
		"password":    u.User.Username(),
	}
	q := u.Query()
	if obfs := q.Get("obfs"); obfs != "" {
		ob["obfs"] = map[string]any{
			"type":     obfs,
			"password": q.Get("obfs-password"),
		}
	}
	if up := q.Get("upmbps"); up != "" {
		if n, err := strconv.Atoi(up); err == nil {
			ob["up_mbps"] = n
		}
	}
	if down := q.Get("downmbps"); down != "" {
		if n, err := strconv.Atoi(down); err == nil {
			ob["down_mbps"] = n
		}
	}
	addTLSAndTransport(ob, q)
	return ProxyOutbound{Type: "hysteria2", Tag: tag, Fields: ob}, nil
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
