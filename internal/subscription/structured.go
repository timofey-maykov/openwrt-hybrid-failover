package subscription

import "strings"

// The input families use different field names. Normalize one server at a time
// into the existing share-URI model; no provider config is executed or applied.
func xrayURIs(out map[string]any, name string) []string {
	if specified(out["proxySettings"]) {
		return nil
	}
	protocol := textValue(out, "protocol")
	kind := protocol
	switch protocol {
	case "vless", "trojan", "shadowsocks", "socks":
	default:
		return nil
	}
	if name == "" {
		name = textValue(out, "tag")
	}
	settings := object(out["settings"])
	stream := object(out["streamSettings"])
	tls := map[string]any{}
	security := textValue(stream, "security")
	switch security {
	case "", "none":
	case "tls", "reality":
		source := object(stream[security+"Settings"])
		for _, key := range []string{"certificates", "pinnedPeerCertificateChainSha256", "verifyPeerCertInNames", "echConfigList"} {
			if specified(source[key]) {
				return nil
			}
		}
		tls = map[string]any{"enabled": true, "server_name": source["serverName"], "insecure": source["allowInsecure"], "alpn": source["alpn"]}
		if fp := textValue(source, "fingerprint"); fp != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
		if security == "reality" {
			tls["reality"] = map[string]any{"enabled": true, "public_key": source["publicKey"], "short_id": source["shortId"]}
		}
	default:
		return nil
	}
	network := textValue(stream, "network")
	tr := map[string]any{}
	switch network {
	case "", "tcp", "raw":
		if h := object(object(stream["tcpSettings"])["header"]); textValue(h, "type") != "" && textValue(h, "type") != "none" {
			return nil
		}
	case "ws", "httpupgrade", "xhttp", "splithttp":
		source := object(stream[network+"Settings"])
		tr = map[string]any{"type": network, "path": source["path"], "headers": source["headers"], "mode": source["mode"]}
		if textValue(source, "host") != "" {
			for key := range object(source["headers"]) {
				if !strings.EqualFold(key, "Host") {
					return nil
				}
			}
			tr["headers"] = map[string]any{"Host": source["host"]}
		}
		if source["extra"] != nil || number(source["maxEarlyData"]) > 0 || textValue(source, "earlyDataHeaderName") != "" {
			return nil
		}
	case "grpc":
		source := object(stream["grpcSettings"])
		if enabled(source["multiMode"]) {
			return nil
		}
		tr = map[string]any{"type": "grpc", "service_name": source["serviceName"], "authority": source["authority"]}
	default:
		return nil
	}
	var result []string
	add := func(server, port any, user map[string]any) {
		node := map[string]any{"type": kind, "server": server, "server_port": port, "tls": tls, "transport": tr,
			"uuid": user["id"], "flow": user["flow"], "encryption": user["encryption"], "password": user["password"], "method": user["method"], "username": user["user"]}
		if uri, ok := singboxURI(node, name); ok {
			result = append(result, uri)
		}
	}
	if protocol == "vless" {
		servers, _ := settings["vnext"].([]any)
		for _, value := range servers {
			server := object(value)
			users, _ := server["users"].([]any)
			for _, user := range users {
				add(server["address"], server["port"], object(user))
			}
		}
	} else {
		servers, _ := settings["servers"].([]any)
		for _, value := range servers {
			server := object(value)
			if protocol == "socks" {
				users, _ := server["users"].([]any)
				if len(users) == 0 {
					add(server["address"], server["port"], nil)
				}
				for _, user := range users {
					add(server["address"], server["port"], object(user))
				}
			} else {
				add(server["address"], server["port"], server)
			}
		}
	}
	return result
}

func clashURI(proxy map[string]any) (string, bool) {
	kind := textValue(proxy, "type")
	switch kind {
	case "ss":
		kind = "shadowsocks"
	case "hy2":
		kind = "hysteria2"
	case "socks5":
		kind = "socks"
	}
	node := map[string]any{"type": kind, "server": proxy["server"], "server_port": proxy["port"], "uuid": proxy["uuid"], "flow": proxy["flow"], "password": proxy["password"], "method": proxy["cipher"], "username": proxy["username"], "plugin": proxy["plugin"]}
	for _, key := range []string{"ca", "ca-str", "client-cert", "client-key", "dialer-proxy"} {
		if proxy[key] != nil {
			return "", false
		}
	}
	tlsEnabled := enabled(proxy["tls"]) || kind == "trojan" || kind == "hysteria2"
	sni := textValue(proxy, "servername")
	if sni == "" {
		sni = textValue(proxy, "sni")
	}
	tls := map[string]any{"enabled": tlsEnabled, "server_name": sni, "insecure": proxy["skip-cert-verify"], "alpn": proxy["alpn"]}
	if fp := textValue(proxy, "client-fingerprint"); fp != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	}
	if r := object(proxy["reality-opts"]); len(r) > 0 {
		tls["enabled"] = true
		tls["reality"] = map[string]any{"enabled": true, "public_key": r["public-key"], "short_id": r["short-id"]}
	}
	node["tls"] = tls
	network := textValue(proxy, "network")
	tr := map[string]any{"type": network}
	switch network {
	case "", "tcp":
	case "ws", "httpupgrade", "xhttp":
		opts := object(proxy[network+"-opts"])
		tr["path"] = opts["path"]
		tr["headers"] = opts["headers"]
		tr["mode"] = opts["mode"]
		if number(opts["max-early-data"]) > 0 || textValue(opts, "early-data-header-name") != "" {
			return "", false
		}
	case "grpc":
		opts := object(proxy["grpc-opts"])
		tr["service_name"] = opts["grpc-service-name"]
	default:
		return "", false
	}
	node["transport"] = tr
	if kind == "hysteria2" {
		if obfs := textValue(proxy, "obfs"); obfs != "" {
			node["obfs"] = map[string]any{"type": obfs, "password": proxy["obfs-password"]}
		}
	}
	return singboxURI(node, textValue(proxy, "name"))
}
