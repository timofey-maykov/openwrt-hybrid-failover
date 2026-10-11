package routing

import "github.com/tmaykov/openwrt-hybrid-failover/internal/secrets"

// MaskURL and MaskSecrets live in core now, so the status files it writes and
// the bot's replies hide links the same way.
func MaskURL(u string) string       { return secrets.MaskURL(u) }
func MaskSecrets(text string) string { return secrets.MaskSecrets(text) }
