package amnezia

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
)

const confScheme = "amneziawg://"

// IsConfURI reports whether raw is an amneziawg:// subscription entry.
func IsConfURI(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), confScheme)
}

// ConfURIToAWG2 converts amneziawg://<base64 wg-quick config>[#name] into awg2://.
// Subscription panels hand out the whole client .conf this way. Only the first
// peer is used, DNS is ignored like for the other awg2 links.
func ConfURIToAWG2(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !IsConfURI(raw) {
		return "", fmt.Errorf("not amneziawg://")
	}
	body := strings.TrimPrefix(raw, confScheme)
	if i := strings.IndexByte(body, '#'); i >= 0 {
		body = body[:i]
	}
	if unescaped, err := url.PathUnescape(body); err == nil {
		body = unescaped
	}
	conf, err := decodeConfBase64(body)
	if err != nil {
		return "", fmt.Errorf("amneziawg:// body is not base64: %w", err)
	}
	iface, peer := parseConfSections(conf)

	host, port, err := net.SplitHostPort(strings.TrimSpace(peer["Endpoint"]))
	if err != nil || host == "" || port == "" {
		return "", fmt.Errorf("amneziawg:// has no usable Endpoint")
	}
	if iface["PrivateKey"] == "" || peer["PublicKey"] == "" {
		return "", fmt.Errorf("amneziawg:// is missing keys")
	}

	q := url.Values{}
	set := func(name, v string) {
		if v != "" {
			q.Set(name, v)
		}
	}
	set("address", iface["Address"])
	set("private_key", iface["PrivateKey"])
	set("mtu", iface["MTU"])
	set("public_key", peer["PublicKey"])
	set("preshared_key", peer["PresharedKey"])
	set("allowed_ips", strings.ReplaceAll(peer["AllowedIPs"], " ", ""))
	set("persistent_keepalive", peer["PersistentKeepalive"])
	for _, key := range []string{"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4", "I1", "I2", "I3", "I4", "I5"} {
		set(strings.ToLower(key), iface[key])
	}
	for confKey, queryKey := range awg31QueryFields {
		set(queryKey, normalizeAWGOnOff(iface[confKey]))
	}
	u := &url.URL{
		Scheme:   "awg2",
		Host:     net.JoinHostPort(host, port),
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

func decodeConfBase64(s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
		if dec, err := enc.DecodeString(strings.TrimRight(s, "=")); err == nil {
			return string(dec), nil
		}
	}
	return "", fmt.Errorf("invalid base64")
}

// parseConfSections returns the [Interface] keys and the keys of the first [Peer].
func parseConfSections(conf string) (iface, peer map[string]string) {
	iface, peer = map[string]string{}, map[string]string{}
	var cur map[string]string
	seenPeer := false
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch strings.ToLower(line) {
		case "[interface]":
			cur = iface
			continue
		case "[peer]":
			if seenPeer {
				cur = nil
			} else {
				cur, seenPeer = peer, true
			}
			continue
		}
		if cur == nil {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			cur[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return iface, peer
}
