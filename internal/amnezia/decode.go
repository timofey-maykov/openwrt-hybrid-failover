package amnezia

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// DecodeVPNURI decodes Amnezia vpn:// export and returns a proxy URI (vless://, trojan://, ss://, or awg2://).
func DecodeVPNURI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "vpn://") {
		return "", fmt.Errorf("not a vpn:// uri")
	}
	payload := strings.TrimPrefix(raw, "vpn://")
	data, err := base64.RawURLEncoding.DecodeString(padB64(payload))
	if err != nil {
		data, err = base64.URLEncoding.DecodeString(padB64(payload))
	}
	if err != nil {
		return "", fmt.Errorf("decode vpn payload: %w", err)
	}
	if len(data) < 8 {
		return "", fmt.Errorf("vpn payload too short")
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[4:]))
	if err != nil {
		return "", fmt.Errorf("zlib: %w", err)
	}
	defer zr.Close()
	var doc map[string]any
	if err := json.NewDecoder(zr).Decode(&doc); err != nil {
		return "", fmt.Errorf("json: %w", err)
	}
	return extractProxyURI(doc)
}

func padB64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	}
	return s
}

func extractProxyURI(doc map[string]any) (string, error) {
	containers, _ := doc["containers"].([]any)
	for _, c := range containers {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if containerName, _ := cm["container"].(string); containerName == "amnezia-awg2" {
			awg, _ := cm["awg"].(map[string]any)
			if uri, err := awg2ContainerToURI(awg); err == nil && uri != "" {
				return uri, nil
			}
		}
		xray, _ := cm["xray"].(map[string]any)
		if xray == nil {
			continue
		}
		last, _ := xray["last_config"].(map[string]any)
		if last == nil {
			if raw, ok := xray["last_config"].(string); ok && raw != "" {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
					last = parsed
				}
			}
		}
		if last == nil {
			continue
		}
		if uri, err := xrayConfigToURI(last); err == nil && uri != "" {
			return uri, nil
		}
	}
	return "", fmt.Errorf("no supported proxy in vpn export")
}

func awg2ContainerToURI(awg map[string]any) (string, error) {
	if awg == nil {
		return "", fmt.Errorf("empty awg container")
	}
	last := awg["last_config"]
	var inner map[string]any
	switch v := last.(type) {
	case string:
		if err := json.Unmarshal([]byte(v), &inner); err != nil {
			return "", err
		}
	case map[string]any:
		inner = v
	default:
		return "", fmt.Errorf("missing awg last_config")
	}
	host := stringField(inner, "hostName")
	if host == "" {
		host = stringField(awg, "hostName")
	}
	port := stringField(inner, "port")
	if port == "" {
		port = stringField(awg, "port")
	}
	if host == "" || port == "" {
		return "", fmt.Errorf("missing awg host/port")
	}
	q := url.Values{}
	if v := stringField(inner, "client_ip"); v != "" {
		q.Set("address", v+"/32")
	}
	if v := stringField(inner, "client_priv_key"); v != "" {
		q.Set("private_key", v)
	}
	if v := stringField(inner, "mtu"); v != "" {
		q.Set("mtu", v)
	}
	if v := stringField(inner, "server_pub_key"); v != "" {
		q.Set("public_key", v)
	}
	if v := stringField(inner, "psk_key"); v != "" {
		q.Set("preshared_key", v)
	}
	q.Set("allowed_ips", "0.0.0.0/0,::/0")
	if v := stringField(inner, "persistent_keep_alive"); v != "" {
		q.Set("persistent_keepalive", v)
	}
	for _, key := range []string{"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5"} {
		if v := stringField(inner, key); v != "" {
			q.Set(strings.ToLower(key), v)
		}
	}
	u := &url.URL{
		Scheme:   "awg2",
		Host:     fmt.Sprintf("%s:%s", host, port),
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprint(t)
	}
}

func xrayConfigToURI(cfg map[string]any) (string, error) {
	outbounds, _ := cfg["outbounds"].([]any)
	if len(outbounds) == 0 {
		return "", fmt.Errorf("no outbounds")
	}
	ob, _ := outbounds[0].(map[string]any)
	proto, _ := ob["protocol"].(string)
	settings, _ := ob["settings"].(map[string]any)
	stream, _ := ob["streamSettings"].(map[string]any)
	switch proto {
	case "vless":
		return buildVLESS(settings, stream)
	case "trojan":
		return buildTrojan(settings, stream)
	case "shadowsocks":
		return buildSS(settings)
	default:
		return "", fmt.Errorf("unsupported protocol %q", proto)
	}
}

func buildVLESS(settings, stream map[string]any) (string, error) {
	vnext, _ := settings["vnext"].([]any)
	if len(vnext) == 0 {
		return "", fmt.Errorf("no vnext")
	}
	vn, _ := vnext[0].(map[string]any)
	addr, _ := vn["address"].(string)
	port, _ := vn["port"].(float64)
	users, _ := vn["users"].([]any)
	if len(users) == 0 {
		return "", fmt.Errorf("no users")
	}
	user, _ := users[0].(map[string]any)
	uuid, _ := user["id"].(string)
	q := streamToQuery(stream)
	if flow, _ := user["flow"].(string); flow != "" {
		q.Set("flow", flow)
	}
	u := &url.URL{
		Scheme:   "vless",
		User:     url.User(uuid),
		Host:     fmt.Sprintf("%s:%d", addr, int(port)),
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

func buildTrojan(settings, stream map[string]any) (string, error) {
	servers, _ := settings["servers"].([]any)
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers")
	}
	srv, _ := servers[0].(map[string]any)
	addr, _ := srv["address"].(string)
	port, _ := srv["port"].(float64)
	pass, _ := srv["password"].(string)
	q := streamToQuery(stream)
	u := &url.URL{
		Scheme:   "trojan",
		User:     url.User(pass),
		Host:     fmt.Sprintf("%s:%d", addr, int(port)),
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

func buildSS(settings map[string]any) (string, error) {
	servers, _ := settings["servers"].([]any)
	if len(servers) == 0 {
		return "", fmt.Errorf("no servers")
	}
	srv, _ := servers[0].(map[string]any)
	method, _ := srv["method"].(string)
	pass, _ := srv["password"].(string)
	addr, _ := srv["address"].(string)
	port, _ := srv["port"].(float64)
	creds := base64.URLEncoding.EncodeToString([]byte(method + ":" + pass))
	u := &url.URL{
		Scheme: "ss",
		User:   url.User(creds),
		Host:   fmt.Sprintf("%s:%d", addr, int(port)),
	}
	return u.String(), nil
}

func streamToQuery(stream map[string]any) url.Values {
	q := url.Values{}
	if stream == nil {
		q.Set("type", "tcp")
		q.Set("security", "none")
		return q
	}
	net, _ := stream["network"].(string)
	if net == "" {
		net = "tcp"
	}
	q.Set("type", net)
	sec, _ := stream["security"].(string)
	if sec == "" {
		sec = "none"
	}
	q.Set("security", sec)
	if sec == "reality" {
		rs, _ := stream["realitySettings"].(map[string]any)
		if rs != nil {
			if v, _ := rs["serverName"].(string); v != "" {
				q.Set("sni", v)
			}
			if v, _ := rs["fingerprint"].(string); v != "" {
				q.Set("fp", v)
			}
			if v, _ := rs["publicKey"].(string); v != "" {
				q.Set("pbk", v)
			}
			if v, ok := rs["shortId"]; ok {
				q.Set("sid", fmt.Sprint(v))
			}
		}
	}
	return q
}

// AWG2InterfaceName returns the Linux interface name for a routing peer section.
func AWG2InterfaceName(section string) string {
	sum := md5.Sum([]byte(section))
	hash := hex.EncodeToString(sum[:])[:8]
	return "pawg" + hash
}

// ParseAWG2URI extracts awg2:// parameters for interface setup.
type AWG2Params struct {
	Host                string
	Port                string
	Address             string
	PrivateKey          string
	PublicKey           string
	MTU                 string
	AllowedIPs          string
	PersistentKeepalive string
	Jc, Jmin, Jmax      string
	S1, S2, S3, S4      string
	H1, H2, H3, H4      string
	I1, I2, I3, I4, I5  string
	PresharedKey        string
}

func ParseAWG2URI(raw string) (AWG2Params, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return AWG2Params{}, err
	}
	if u.Scheme != "awg2" {
		return AWG2Params{}, fmt.Errorf("not awg2://")
	}
	q := u.Query()
	p := AWG2Params{
		Host:                u.Hostname(),
		Port:                u.Port(),
		Address:             q.Get("address"),
		PrivateKey:          q.Get("private_key"),
		PublicKey:           q.Get("public_key"),
		MTU:                 q.Get("mtu"),
		AllowedIPs:          q.Get("allowed_ips"),
		PersistentKeepalive: q.Get("persistent_keepalive"),
		Jc:                  q.Get("jc"), Jmin: q.Get("jmin"), Jmax: q.Get("jmax"),
		S1: q.Get("s1"), S2: q.Get("s2"), S3: q.Get("s3"), S4: q.Get("s4"),
		H1: q.Get("h1"), H2: q.Get("h2"), H3: q.Get("h3"), H4: q.Get("h4"),
		I1: q.Get("i1"), I2: q.Get("i2"), I3: q.Get("i3"), I4: q.Get("i4"), I5: q.Get("i5"),
		PresharedKey: q.Get("preshared_key"),
	}
	if p.Address == "" {
		p.Address = "10.255.255.2/32"
	}
	if p.AllowedIPs == "" {
		p.AllowedIPs = "0.0.0.0/0,::/0"
	}
	if p.PersistentKeepalive == "" {
		p.PersistentKeepalive = "25"
	}
	if p.Host == "" || p.Port == "" || p.PrivateKey == "" || p.PublicKey == "" {
		return AWG2Params{}, fmt.Errorf("invalid awg2://: missing host/port/keys")
	}
	return p, nil
}
