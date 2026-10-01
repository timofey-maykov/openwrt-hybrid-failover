// Package channels gives every proxy link of a routing section a stable id and
// a human name, so service lists can be bound to a channel that survives
// reordering of the link list (the engine tags, section-N-out, follow the
// position and would not).
package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// Channel is one link of a section as the user sees it.
type Channel struct {
	ID      string // stable id, see ID
	Link    string
	Index   int    // 1-based position in the section's link list
	Name    string // user label or a generated one
	Primary bool   // the VPN interface of a vpn section
}

// PrimaryID is the id of the VPN interface channel of a vpn section.
const PrimaryID = "vpn"

// ID returns a short stable id for a link: what identifies the server and the
// account (host, port, key or password), not the tuning parameters, so editing
// an obfuscation option keeps the id. Two links to the same AWG peer with
// different IPs share the id, as they share the interface.
func ID(link string) string {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "vpn://") {
		if decoded, err := amnezia.DecodeVPNURI(link); err == nil {
			link = decoded
		}
	}
	key := identity(link)
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:8]
}

func identity(link string) string {
	if strings.HasPrefix(link, "awg2://") {
		if p, err := amnezia.ParseAWG2URI(link); err == nil && p.PublicKey != "" {
			return "awg2|" + p.PublicKey
		}
	}
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return link
	}
	user := ""
	if u.User != nil {
		user = u.User.String()
	}
	return strings.ToLower(u.Scheme) + "|" + user + "@" + strings.ToLower(u.Host)
}

// DefaultName is shown when the user did not name the channel: protocol and
// host, e.g. "hysteria2 77.110.127.13".
func DefaultName(link string) string {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "vpn://") {
		if decoded, err := amnezia.DecodeVPNURI(link); err == nil {
			link = decoded
		}
	}
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		if len(link) > 24 {
			return link[:24]
		}
		return link
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "hy2":
		scheme = "hysteria2"
	case "awg2":
		scheme = "AWG"
	}
	return scheme + " " + u.Hostname()
}

// Names reads `list channel_names 'id=Name'` of a section.
func Names(sec *uci.Section) map[string]string {
	out := make(map[string]string)
	if sec == nil {
		return out
	}
	for _, raw := range sec.GetList("channel_names") {
		id, name, ok := strings.Cut(raw, "=")
		id = strings.TrimSpace(id)
		name = strings.TrimSpace(name)
		if !ok || id == "" || name == "" {
			continue
		}
		out[id] = name
	}
	return out
}

// Links returns the proxy links of a section in order: urltest links of a
// proxy section, backups of a vpn section.
func Links(sec *uci.Section) []string {
	if sec == nil {
		return nil
	}
	switch sec.Get("connection_type", "") {
	case "vpn":
		if !sec.GetBool("failover_vpn_enabled", false) {
			return nil
		}
		return sec.GetList("failover_proxy_links")
	case "proxy":
		switch sec.Get("proxy_config_type", "url") {
		case "urltest":
			return sec.GetList("urltest_proxy_links")
		case "url":
			if s := strings.TrimSpace(sec.Get("proxy_string", "")); s != "" {
				return []string{s}
			}
		}
	}
	return nil
}

// List returns the channels of a section. Links that share an id (alternate
// IPs of one AWG peer) are reported once, at the first position.
func List(sec *uci.Section) []Channel {
	if sec == nil {
		return nil
	}
	names := Names(sec)
	var out []Channel
	if sec.Get("connection_type", "") == "vpn" {
		name := names[PrimaryID]
		if name == "" {
			name = "VPN " + sec.Get("interface", "")
		}
		out = append(out, Channel{ID: PrimaryID, Name: name, Primary: true})
	}
	seen := make(map[string]bool)
	for i, link := range Links(sec) {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		id := ID(link)
		if seen[id] {
			continue
		}
		seen[id] = true
		name := names[id]
		if name == "" {
			name = DefaultName(link)
		}
		out = append(out, Channel{ID: id, Link: link, Index: i + 1, Name: name})
	}
	return out
}
