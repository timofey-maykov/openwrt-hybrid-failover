package probe

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// ChannelTimeout is per-channel budget for live probes.
const ChannelTimeout = 12 * time.Second

// Delayer returns outbound delay in milliseconds (Clash API or native engine control).
type Delayer interface {
	ProxyDelay(ctx context.Context, tag, testURL string) (int, error)
}

// Outbound tries Delayer /delay; for Direct outbounds falls back to curl via bind interface.
func Outbound(ctx context.Context, delayer Delayer, tag, testURL, proxyType, bindIface string) (delay int, ok bool, detail string) {
	if testURL == "" {
		testURL = "https://www.gstatic.com/generate_204"
	}
	delay, err := delayer.ProxyDelay(ctx, tag, testURL)
	if err == nil && delay > 0 {
		return delay, true, ""
	}
	if err != nil {
		detail = classifyProbeError(err.Error())
	}
	if bindIface == "" || !isDirectType(proxyType) {
		if detail == "" {
			detail = "no delay data from Clash API"
		}
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
		detail += classifyProbeError(err2.Error())
	}
	return 0, false, detail
}

func classifyProbeError(msg string) string {
	msg = strings.ToLower(msg)
	switch {
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "timeout"
	case strings.Contains(msg, "dns"), strings.Contains(msg, "resolve"), strings.Contains(msg, "lookup"):
		return "DNS failure"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "connect"):
		return "connection failed"
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "400"):
		return "probe rejected"
	default:
		return strings.TrimSpace(msg)
	}
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

// IfaceLinkUp reports whether a network interface has carrier (LOWER_UP).
func IfaceLinkUp(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	out, err := exec.Command("ip", "link", "show", "dev", name).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "LOWER_UP")
}

// BindIfaceForChannel resolves Linux interface for a Direct-style outbound tag.
func BindIfaceForChannel(section string, sec *uci.Section, tag string) string {
	if sec == nil || section == "" {
		return ""
	}
	if tag == singbox.AWGTag(section) {
		return sec.Get("interface", "")
	}
	if sec.Get("connection_type", "") == "vpn" {
		return bindIfaceForFailoverLink(section, sec, tag)
	}
	if sec.Get("connection_type", "") == "proxy" {
		return bindIfaceForProxyLink(section, sec, tag)
	}
	return ""
}

func bindIfaceForFailoverLink(section string, sec *uci.Section, tag string) string {
	links := sec.GetList("failover_proxy_links")
	for i, link := range links {
		if singbox.PeerTag(section, i+1) != tag {
			continue
		}
		return bindIfaceForURI(section, i+1, link)
	}
	return ""
}

func bindIfaceForProxyLink(section string, sec *uci.Section, tag string) string {
	links := sec.GetList("urltest_proxy_links")
	for i, link := range links {
		if singbox.PeerTag(section, i+1) != tag {
			continue
		}
		return bindIfaceForURI(section, i+1, link)
	}
	return ""
}

func bindIfaceForURI(section string, idx int, link string) string {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "awg2://") {
		return amnezia.AWG2InterfaceName(fmt.Sprintf("%s-%d", section, idx))
	}
	if strings.HasPrefix(link, "vpn://") {
		decoded, err := amnezia.DecodeVPNURI(link)
		if err == nil && strings.HasPrefix(decoded, "awg2://") {
			return amnezia.AWG2InterfaceName(fmt.Sprintf("%s-%d", section, idx))
		}
	}
	return ""
}
