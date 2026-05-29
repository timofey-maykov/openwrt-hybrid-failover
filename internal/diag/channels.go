package diag

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/clash"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/failover"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/policy"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/probe"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// ChannelStatus is one failover / urltest candidate from Clash API.
type ChannelStatus struct {
	Name      string `json:"name"`
	Display   string `json:"display,omitempty"`
	Type      string `json:"type,omitempty"`
	DelayMs   int    `json:"delay_ms,omitempty"`
	Available bool   `json:"available"`
	Selected  bool    `json:"selected"`
	Detail    string `json:"detail,omitempty"`
	Probed    bool   `json:"probed,omitempty"`
}

// FailoverInfo summarizes urltest / failover UCI for the main routing section.
type FailoverInfo struct {
	Section       string `json:"section"`
	Policy        string `json:"policy,omitempty"`
	CheckInterval string `json:"check_interval,omitempty"`
	Tolerance     string `json:"tolerance,omitempty"`
	IdleTimeout   string `json:"idle_timeout,omitempty"`
	TestingURL    string `json:"testing_url,omitempty"`
	SelectorNow   string `json:"selector_now,omitempty"`
	URLTestNow    string `json:"urltest_now,omitempty"`
}

func failoverInfoFromSection(section string, sec *uci.Section) *FailoverInfo {
	if sec == nil {
		return &FailoverInfo{Section: section}
	}
	info := &FailoverInfo{
		Section:       section,
		Policy:        string(policy.Normalize(sec.Get("failover_policy", ""))),
		CheckInterval: sec.Get("urltest_check_interval", ""),
		Tolerance:     sec.Get("urltest_tolerance", ""),
		IdleTimeout:   sec.Get("urltest_idle_timeout", ""),
		TestingURL:    sec.Get("urltest_testing_url", ""),
	}
	return info
}

// EnrichReport adds per-channel status and failover parameters when Clash API is up.
func EnrichReport(r Report, clashURL, mainSection string, sec *uci.Section) Report {
	if !r.ClashOK || mainSection == "" {
		return r
	}
	cli := clash.New(clashURL, 8*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	resp, err := cli.Proxies(ctx)
	if err != nil {
		r.Errors = append(r.Errors, "channels: "+err.Error())
		return r
	}

	selectorTag := singbox.OutboundTag(mainSection)
	if p, ok := resp.Proxies[selectorTag]; ok {
		r.Failover = failoverInfoFromSection(mainSection, sec)
		r.Failover.SelectorNow = p.Now
		if ut, ok := resp.Proxies[singbox.URLTestTag(mainSection)]; ok {
			r.Failover.URLTestNow = ut.Now
		}
	}

	active := r.ActiveOutbound
	if active == "" {
		active = r.Failover.SelectorNow
	}
	r.Channels = buildChannels(resp, mainSection, sec, active)
	if states, err := failover.ReadRuntimeState(); err == nil && len(states) > 0 {
		r.Controller = MapControllerStates(states)
	}
	return r
}

// MapControllerStates converts persisted failover runtime to API DTOs.
func MapControllerStates(states []failover.SectionRuntime) []ControllerSection {
	out := make([]ControllerSection, 0, len(states))
	for _, st := range states {
		out = append(out, ControllerSection{
			Section:       st.Section,
			Policy:        st.Policy,
			Mode:          st.Mode,
			Active:        st.Active,
			PrimaryOK:     st.PrimaryOK,
			PrimaryDelay:  st.PrimaryDelay,
			FailStreak:    st.FailStreak,
			RecoverStreak: st.RecoverStreak,
			LastProbeAt:   formatTimeRFC(st.LastProbeAt),
			LastSwitchAt:  formatTimeRFC(st.LastSwitchAt),
			ActiveSince:   formatTimeRFC(st.ActiveSince),
			LastError:     st.LastError,
		})
	}
	return out
}

// ProbeChannels runs live delay probes for each listed channel (slower, like bot /health).
func ProbeChannels(r Report, clashURL, mainSection string, sec *uci.Section) Report {
	if !r.ClashOK {
		return r
	}
	testURL := "https://www.gstatic.com/generate_204"
	if sec != nil {
		if u := sec.Get("urltest_testing_url", ""); u != "" {
			testURL = u
		}
	}
	cli := clash.New(clashURL, 6*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if len(r.Channels) == 0 {
		r = EnrichReport(r, clashURL, mainSection, sec)
	}
	for i := range r.Channels {
		ch := &r.Channels[i]
		iface := probe.BindIfaceForChannel(mainSection, sec, ch.Name)
		pctx, pcancel := context.WithTimeout(ctx, probe.ChannelTimeout)
		delay, ok, detail := probe.Outbound(pctx, cli, ch.Name, testURL, ch.Type, iface)
		pcancel()
		ch.Probed = true
		ch.DelayMs = delay
		ch.Available = ok
		ch.Detail = detail
	}
	return r
}

func buildChannels(resp clash.ProxiesResponse, section string, sec *uci.Section, activeOutbound string) []ChannelStatus {
	selectorTag := singbox.OutboundTag(section)
	urltestTag := singbox.URLTestTag(section)

	if sec != nil && sec.Get("connection_type", "") == "vpn" && sec.GetBool("failover_vpn_enabled", false) {
		return channelsFromURLTest(resp, section, sec, selectorTag, urltestTag, activeOutbound, true)
	}
	if sec != nil && sec.Get("connection_type", "") == "proxy" && sec.Get("proxy_config_type", "") == "urltest" {
		return channelsFromURLTest(resp, section, sec, selectorTag, urltestTag, activeOutbound, false)
	}

	return channelsFromLeafList(resp, section, activeOutbound)
}

func channelsFromURLTest(resp clash.ProxiesResponse, section string, sec *uci.Section, selectorTag, urltestTag, activeOutbound string, vpnFailover bool) []ChannelStatus {
	ut, ok := resp.Proxies[urltestTag]
	if !ok || len(ut.All) == 0 {
		return channelsFromLeafList(resp, section, activeOutbound)
	}
	selectorNow := ""
	if sel, ok := resp.Proxies[selectorTag]; ok {
		selectorNow = sel.Now
	}

	var channels []ChannelStatus
	managedVPN := vpnFailover && sec != nil &&
		policy.Normalize(sec.Get("failover_policy", "")) != policy.Fastest

	if managedVPN {
		awgTag := singbox.AWGTag(section)
		if p, ok := resp.Proxies[awgTag]; ok {
			iface := sec.Get("interface", awgTag)
			channels = append(channels, channelFromProxy(awgTag, p, iface+" (primary VPN)", selectorNow, activeOutbound, false))
		}
	}

	channels = append(channels, channelFromProxy(urltestTag, ut, "URLTest (резервы)", selectorNow, activeOutbound, false))

	for i, member := range ut.All {
		p, ok := resp.Proxies[member]
		if !ok {
			continue
		}
		display := member
		if vpnFailover {
			links := sec.GetList("failover_proxy_links")
			if i < len(links) {
				display = shortProxyLabel(links[i]) + " (" + member + ")"
			}
		} else {
			links := sec.GetList("urltest_proxy_links")
			if i < len(links) {
				display = shortProxyLabel(links[i]) + " (" + member + ")"
			}
		}
		channels = append(channels, channelFromProxy(member, p, display, selectorNow, activeOutbound, false))
	}
	return channels
}

func channelsFromLeafList(resp clash.ProxiesResponse, section, activeOutbound string) []ChannelStatus {
	prefix := section + "-"
	var names []string
	for name := range resp.Proxies {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, "-out") &&
			name != singbox.OutboundTag(section) && name != singbox.URLTestTag(section) {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var channels []ChannelStatus
	for _, name := range names {
		p := resp.Proxies[name]
		channels = append(channels, channelFromProxy(name, p, name, "", activeOutbound, false))
	}
	return channels
}

func channelFromProxy(name string, p clash.ProxyInfo, display, selectorNow, activeOutbound string, probed bool) ChannelStatus {
	delay := p.LastDelayMS()
	selected := activeOutbound == name || selectorNow == name
	ch := ChannelStatus{
		Name:      name,
		Display:   display,
		Type:      p.Type,
		DelayMs:   delay,
		Available: delay > 0 || p.Type == "direct",
		Selected:  selected,
		Probed:    probed,
	}
	if !ch.Available && delay == 0 {
		ch.Detail = "нет данных о задержке (нажмите «Проверить каналы»)"
	}
	return ch
}

func formatTimeRFC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func shortProxyLabel(uri string) string {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return ""
	}
	if i := strings.Index(uri, "@"); i >= 0 && i+1 < len(uri) {
		host := uri[i+1:]
		if j := strings.IndexAny(host, "/?#"); j >= 0 {
			host = host[:j]
		}
		if host != "" {
			return host
		}
	}
	if len(uri) > 48 {
		return uri[:45] + "..."
	}
	return uri
}
