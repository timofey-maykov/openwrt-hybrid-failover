package probe

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// ChannelTimeout is per-channel budget for live probes.
const ChannelTimeout = 12 * time.Second

// Outbound tries Clash /delay; for Direct outbounds falls back to curl via bind interface.
func Outbound(ctx context.Context, cli *clash.Client, tag, testURL, proxyType, bindIface string) (delay int, ok bool, detail string) {
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	delay, err := cli.ProxyDelay(ctx, tag, testURL)
	if err == nil && delay > 0 {
		return delay, true, ""
	}
	if err != nil {
		detail = err.Error()
	}
	if bindIface == "" || !isDirectType(proxyType) {
		return 0, false, detail
	}
	d, err2 := viaInterface(ctx, bindIface, testURL)
	if err2 == nil && d > 0 {
		return d, true, "via " + bindIface
	}
	if err2 != nil {
		if detail != "" {
			detail += "; "
		}
		detail += err2.Error()
	}
	return 0, false, detail
}

func isDirectType(proxyType string) bool {
	return strings.EqualFold(strings.TrimSpace(proxyType), "direct")
}

func viaInterface(ctx context.Context, iface, testURL string) (int, error) {
	if iface == "" {
		return 0, fmt.Errorf("empty interface")
	}
	args := []string{
		"curl", "-m", "8", "-sSf", "-o", "/dev/null",
		"-w", "%{time_total}",
		"--interface", iface,
		testURL,
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("curl --interface %s: %w: %s", iface, err, strings.TrimSpace(string(out)))
	}
	sec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || sec <= 0 {
		return 0, fmt.Errorf("invalid curl timing: %q", string(out))
	}
	return int(sec * 1000), nil
}

// BindIfaceForChannel resolves Linux interface for a Direct-style outbound tag.
func BindIfaceForChannel(section string, sec *uci.Section, tag string) string {
	if sec == nil || section == "" {
		return ""
	}
	if tag == singbox.AWGTag(section) {
		return sec.Get("interface", "")
	}
	links := sec.GetList("failover_proxy_links")
	for i, link := range links {
		if singbox.PeerTag(section, i+1) != tag {
			continue
		}
		link = strings.TrimSpace(link)
		if strings.HasPrefix(link, "awg2://") {
			return amnezia.AWG2InterfaceName(fmt.Sprintf("%s-%d", section, i+1))
		}
		if strings.HasPrefix(link, "vpn://") {
			decoded, err := amnezia.DecodeVPNURI(link)
			if err == nil && strings.HasPrefix(decoded, "awg2://") {
				return amnezia.AWG2InterfaceName(fmt.Sprintf("%s-%d", section, i+1))
			}
		}
	}
	return ""
}
