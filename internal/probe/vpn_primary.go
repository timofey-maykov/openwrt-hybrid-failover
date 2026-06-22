package probe

import (
	"context"
	"fmt"
	"time"

)

// PrimaryVPN probes a bind-interface VPN primary.
// WireGuard/AmneziaWG: require a fresh handshake; HTTP probe is best-effort (split-tunnel may block gstatic).
// Other interfaces: require a successful HTTP probe.
func PrimaryVPN(ctx context.Context, delayer Delayer, primaryTag, testURL, bindIface string) (delay int, ok bool, detail string) {
	if bindIface != "" && !IfaceLinkUp(bindIface) {
		return 0, false, "interface down"
	}
	if bindIface != "" {
		if hs, wgOK := checkWGHandshake(bindIface, DefaultWGHandshakeMaxAge); wgOK {
			if !hs.fresh {
				return 0, false, hs.detail
			}
			delay, ok, detail = Outbound(ctx, delayer, primaryTag, testURL, "direct", bindIface)
			if ok {
				return delay, true, detail
			}
			return 0, true, "wireguard handshake OK"
		}
	}
	return Outbound(ctx, delayer, primaryTag, testURL, "direct", bindIface)
}

type wgHSResult struct {
	fresh  bool
	detail string
}

func checkWGHandshake(iface string, maxAge time.Duration) (wgHSResult, bool) {
	if maxAge <= 0 {
		maxAge = DefaultWGHandshakeMaxAge
	}
	out, err := wgShow(iface, "latest-handshakes")
	if err != nil {
		return wgHSResult{}, false
	}
	last, ok := parseLatestHandshake(out)
	if !ok {
		return wgHSResult{detail: "no wireguard handshake"}, true
	}
	age := time.Since(last)
	if age > maxAge {
		return wgHSResult{
			detail: fmt.Sprintf("wireguard handshake stale (%s ago)", formatDurationShort(age)),
		}, true
	}
	return wgHSResult{fresh: true}, true
}
