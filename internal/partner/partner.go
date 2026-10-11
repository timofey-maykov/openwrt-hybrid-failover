// Package partner knows where a user without a VPN can get a subscription
// that works with this router out of the box.
package partner

import (
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// DefaultURL opens the PulseVPN bot; its subscriptions are AmneziaWG links
// the router reads as they are.
const DefaultURL = "https://t.me/vpnpulsemain_bot?start=router"

// Name is how the provider is called in the UI.
const Name = "PulseVPN"

// URL returns settings.partner_url, the default when it is unset, and "" when
// it is set to off.
func URL(settings *uci.Section) string {
	if settings == nil {
		return DefaultURL
	}
	v := strings.TrimSpace(settings.Get("partner_url", ""))
	switch strings.ToLower(v) {
	case "":
		return DefaultURL
	case "off", "0", "none":
		return ""
	}
	return v
}
