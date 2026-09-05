package diag

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/delayhistory"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/failover"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/probe"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// EnrichNativeReport fills failover summary and channel list for engine_mode=native.
func EnrichNativeReport(r Report, mainSection string, sec *uci.Section, states []failover.SectionRuntime) Report {
	if mainSection == "" {
		return r
	}
	if sec == nil {
		return r
	}
	active := normalizeSectionActive(mainSection, activeFromController(states, mainSection))
	if active == "" {
		active = normalizeSectionActive(mainSection, r.ActiveOutbound)
	}
	if active != "" {
		r.ActiveOutbound = active
	}

	urltestMember := urlTestMemberFromStates(states, mainSection)
	if urltestMember == "" {
		urltestMember = engine.URLTestMemberFromSnapshot(mainSection)
	}

	r.Failover = failoverInfoFromSection(mainSection, sec)
	if active != "" {
		r.Failover.SelectorNow = active
		if urltestMember != "" {
			r.Failover.URLTestNow = urltestMember
		} else if active == singbox.URLTestTag(mainSection) {
			r.Failover.URLTestNow = active
		} else if active != singbox.AWGTag(mainSection) {
			r.Failover.URLTestNow = active
		}
	}

	r.Channels = buildNativeChannels(mainSection, sec, active, urltestMember, false)
	return r
}

// ProbeNativeChannels refreshes channel delays. Uses the running engine when available
// to avoid spawning a second outbound registry (large memory spike on small routers).
func ProbeNativeChannels(r Report, mainSection string, sec *uci.Section) Report {
	if mainSection == "" || sec == nil {
		return r
	}
	if len(r.Channels) == 0 {
		r = EnrichNativeReport(r, mainSection, sec, nil)
	}
	if engine.Alive() {
		r = probeNativeFromEngine(r, mainSection, sec)
	} else {
		r = probeNativeWithRegistry(r, mainSection, sec)
	}
	overlayReportAWG2(r.Channels, mainSection, sec)
	return r
}

func probeNativeFromEngine(r Report, mainSection string, sec *uci.Section) Report {
	testURL := sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204")
	delays := engineDelays()
	backend := failoverEngineDelayer{}
	ctx, cancel := context.WithTimeout(context.Background(), probe.UITotalTimeout)
	defer cancel()

	var reprobe []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range r.Channels {
		ch := &r.Channels[i]
		if ch.Name == singbox.AWGTag(mainSection) {
			i := i
			name := ch.Name
			iface := sec.Get("interface", "")
			wg.Add(1)
			go func() {
				defer wg.Done()
				pctx, pcancel := context.WithTimeout(ctx, probe.UIChannelTimeout)
				delay, ok, detail := probe.PrimaryVPN(pctx, backend, name, testURL, iface)
				pcancel()
				mu.Lock()
				r.Channels[i].Probed = true
				r.Channels[i].DelayMs = delay
				r.Channels[i].Available = ok
				r.Channels[i].Detail = detail
				mu.Unlock()
			}()
			continue
		}
		if ch.Type == "urltest" {
			if snap, ok := delays[ch.Name]; ok && snap.OK && snap.DelayMs > 0 {
				ch.Probed = true
				ch.DelayMs = snap.DelayMs
				ch.Available = true
				ch.Detail = ""
			} else {
				reprobe = append(reprobe, i)
			}
			continue
		}
		if snap, ok := delays[ch.Name]; ok && snap.OK && snap.DelayMs > 0 {
			ch.Probed = true
			ch.DelayMs = snap.DelayMs
			ch.Available = true
			ch.Detail = ""
			continue
		}
		reprobe = append(reprobe, i)
	}
	wg.Wait()
	if len(reprobe) > 0 {
		r = probeNativeChannelsActive(r, mainSection, sec, reprobe)
	}
	if r.Failover != nil {
		if member := engine.URLTestMemberFromSnapshot(mainSection); member != "" {
			r.Failover.URLTestNow = member
		}
	}
	return r
}

func probeNativeChannelsActive(r Report, mainSection string, sec *uci.Section, indices []int) Report {
	pkg, err := uci.Load(paths.UCIConfig)
	if err != nil {
		return r
	}
	p, err := engine.CompilePlan(pkg)
	if err != nil {
		return r
	}
	reg, err := outbound.NewRegistry(p.Outbounds)
	if err != nil {
		return r
	}
	defer reg.Stop()

	testURL := sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204")
	ctx, cancel := context.WithTimeout(context.Background(), probe.UITotalTimeout)
	defer cancel()

	var (
		mu         sync.Mutex
		wg         sync.WaitGroup
		bestMember string
		bestDelay  = -1
	)
	for _, i := range indices {
		i := i
		name := r.Channels[i].Name
		chType := r.Channels[i].Type
		wg.Add(1)
		go func() {
			defer wg.Done()
			pctx, pcancel := context.WithTimeout(ctx, probe.UIChannelTimeout)
			var delay int
			var ok bool
			var detail string
			var member string
			if name == singbox.AWGTag(mainSection) {
				iface := sec.Get("interface", "")
				delay, ok, detail = probe.PrimaryVPN(pctx, registryDelayer{reg: reg}, name, testURL, iface)
			} else if chType == "urltest" {
				delay, ok, detail, member = probeURLTestGroup(pctx, reg, p, name, testURL)
			} else {
				delay, ok, detail = probeRegistryOutbound(pctx, reg, name, testURL)
			}
			pcancel()
			mu.Lock()
			r.Channels[i].Probed = true
			r.Channels[i].DelayMs = delay
			r.Channels[i].Available = ok
			r.Channels[i].Detail = detail
			if member != "" && (bestDelay < 0 || delay < bestDelay) {
				bestMember = member
				bestDelay = delay
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if bestMember != "" {
		urltestGroup := singbox.URLTestTag(mainSection)
		for i := range r.Channels {
			r.Channels[i].Selected = channelSelected(r.Channels[i].Name, r.ActiveOutbound, bestMember, urltestGroup)
		}
		if r.Failover != nil {
			r.Failover.URLTestNow = bestMember
		}
	}
	return r
}

func probeNativeWithRegistry(r Report, mainSection string, sec *uci.Section) Report {
	indices := make([]int, len(r.Channels))
	for i := range r.Channels {
		indices[i] = i
	}
	return probeNativeChannelsActive(r, mainSection, sec, indices)
}

type registryDelayer struct {
	reg *outbound.Registry
}

type failoverEngineDelayer struct{}

func (failoverEngineDelayer) ProxyDelay(ctx context.Context, tag, testURL string) (int, error) {
	b := failover.NewEngineBackend()
	return b.ProxyDelay(ctx, tag, testURL)
}

func (d registryDelayer) ProxyDelay(ctx context.Context, tag, testURL string) (int, error) {
	delay, ok, detail := probeRegistryOutbound(ctx, d.reg, tag, testURL)
	if !ok {
		if detail == "" {
			detail = "probe failed"
		}
		return 0, fmt.Errorf("%s", detail)
	}
	return delay, nil
}

func probeRegistryOutbound(ctx context.Context, reg *outbound.Registry, tag, testURL string) (delay int, ok bool, detail string) {
	h, err := reg.Handler(tag)
	if err != nil {
		return 0, false, err.Error()
	}
	start := time.Now()
	conn, err := h.DialTCP(ctx, "tcp", hostPortFromURL(testURL))
	if err != nil {
		return 0, false, err.Error()
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms <= 0 {
		ms = 1
	}
	return ms, true, ""
}

func probeURLTestGroup(ctx context.Context, reg *outbound.Registry, p *engine.Plan, tag, testURL string) (delay int, ok bool, detail, bestMember string) {
	for _, ob := range p.Outbounds {
		if ob.Tag != tag || ob.Kind != engine.OutboundURLTest {
			continue
		}
		type memberResult struct {
			tag    string
			ms     int
			ok     bool
			detail string
		}
		results := make([]memberResult, len(ob.Members))
		var wg sync.WaitGroup
		for i, member := range ob.Members {
			i, member := i, member
			wg.Add(1)
			go func() {
				defer wg.Done()
				ms, memberOK, memberDetail := probeRegistryOutbound(ctx, reg, member, testURL)
				results[i] = memberResult{tag: member, ms: ms, ok: memberOK, detail: memberDetail}
			}()
		}
		wg.Wait()
		bestMs := -1
		for _, res := range results {
			if !res.ok {
				continue
			}
			if bestMs < 0 || res.ms < bestMs {
				bestMs = res.ms
				bestMember = res.tag
				delay = res.ms
				ok = true
				detail = ""
			}
		}
		if !ok {
			return 0, false, "all urltest members failed", ""
		}
		return delay, ok, detail, bestMember
	}
	return 0, false, "urltest group not found", ""
}

func normalizeSectionActive(section, active string) string {
	if active == "" || active == singbox.OutboundTag(section) {
		return ""
	}
	return active
}

func activeFromController(states []failover.SectionRuntime, section string) string {
	for _, st := range states {
		if st.Section == section && st.Active != "" {
			return st.Active
		}
	}
	if len(states) == 1 && states[0].Active != "" {
		return states[0].Active
	}
	return ""
}

func urlTestMemberFromStates(states []failover.SectionRuntime, section string) string {
	for _, st := range states {
		if st.Section == section && st.URLTestMember != "" {
			return st.URLTestMember
		}
	}
	return ""
}

func buildNativeChannels(section string, sec *uci.Section, activeOutbound, urltestMember string, probed bool) []ChannelStatus {
	if sec == nil {
		return nil
	}
	delayCache := nativeDelayCache()

	conn := sec.Get("connection_type", "")
	if conn == "vpn" && sec.GetBool("failover_vpn_enabled", false) {
		return nativeVPNChannels(section, sec, activeOutbound, urltestMember, probed, delayCache)
	}
	if conn == "proxy" && sec.Get("proxy_config_type", "") == "urltest" {
		return nativeProxyURLTestChannels(section, sec, activeOutbound, urltestMember, probed, delayCache)
	}
	return nil
}

func nativeVPNChannels(section string, sec *uci.Section, activeOutbound, urltestMember string, probed bool, delays map[string]delayhistory.Sample) []ChannelStatus {
	var channels []ChannelStatus
	urltestGroup := singbox.URLTestTag(section)
	awgTag := singbox.AWGTag(section)
	iface := sec.Get("interface", awgTag)
	channels = append(channels, nativeChannel(awgTag, iface+" (primary VPN)", "direct", activeOutbound, urltestMember, urltestGroup, probed, delays[awgTag]))

	channels = append(channels, nativeChannel(urltestGroup, "URLTest (резервы)", "urltest", activeOutbound, urltestMember, urltestGroup, probed, delays[urltestGroup]))

	seenAWG := make(map[string]struct{})
	links := sec.GetList("failover_proxy_links")
	for i, link := range links {
		if skipDuplicateAWG2Link(seenAWG, link) {
			continue
		}
		tag := singbox.PeerTag(section, i+1)
		display := shortProxyLabel(link) + " (" + tag + ")"
		ch := nativeChannel(tag, display, proxyScheme(link), activeOutbound, urltestMember, urltestGroup, probed, delays[tag])
		applyAWG2ChannelOverlay(&ch, section, sec, tag)
		channels = append(channels, ch)
	}
	return channels
}

func nativeProxyURLTestChannels(section string, sec *uci.Section, activeOutbound, urltestMember string, probed bool, delays map[string]delayhistory.Sample) []ChannelStatus {
	var channels []ChannelStatus
	urltestGroup := singbox.URLTestTag(section)
	channels = append(channels, nativeChannel(urltestGroup, "URLTest", "urltest", activeOutbound, urltestMember, urltestGroup, probed, delays[urltestGroup]))
	seenAWG := make(map[string]struct{})
	links := sec.GetList("urltest_proxy_links")
	for i, link := range links {
		if skipDuplicateAWG2Link(seenAWG, link) {
			continue
		}
		tag := singbox.PeerTag(section, i+1)
		display := shortProxyLabel(link) + " (" + tag + ")"
		ch := nativeChannel(tag, display, proxyScheme(link), activeOutbound, urltestMember, urltestGroup, probed, delays[tag])
		applyAWG2ChannelOverlay(&ch, section, sec, tag)
		channels = append(channels, ch)
	}
	return channels
}

func overlayReportAWG2(channels []ChannelStatus, section string, sec *uci.Section) {
	for i := range channels {
		applyAWG2ChannelOverlay(&channels[i], section, sec, channels[i].Name)
	}
}

func applyAWG2ChannelOverlay(ch *ChannelStatus, section string, sec *uci.Section, tag string) {
	if ch == nil || ch.Type != "awg2" {
		return
	}
	iface := probe.BindIfaceForChannel(section, sec, tag)
	ok, delay, detail := probe.OverlayAWG2Health(ch.DelayMs, probe.ReadAWGHandshake(iface))
	ch.Available = ok
	ch.DelayMs = delay
	ch.Detail = detail
}

func nativeChannel(name, display, typ, activeOutbound, urltestMember, urltestGroup string, probed bool, last delayhistory.Sample) ChannelStatus {
	ch := ChannelStatus{
		Name:     name,
		Display:  display,
		Type:     typ,
		Selected: channelSelected(name, activeOutbound, urltestMember, urltestGroup),
		Probed:   probed,
	}
	if !engine.Alive() && !probed {
		ch.Detail = "engine not running"
		return ch
	}
	if last.Time.IsZero() {
		if snap, ok := engineDelays()[name]; ok {
			ch.DelayMs = snap.DelayMs
			ch.Available = snap.OK && snap.DelayMs > 0
		}
		if typ == "urltest" && urltestMember != "" {
			if snap, ok := engineDelays()[urltestMember]; ok && snap.OK {
				ch.Available = true
				if ch.DelayMs == 0 {
					ch.DelayMs = snap.DelayMs
				}
			}
		}
		if !probed && !ch.Available && ch.DelayMs == 0 {
			if !last.Time.IsZero() {
				ch.Detail = "unavailable"
			} else if snap, ok := engineDelays()[name]; ok && !snap.OK {
				ch.Detail = "unavailable"
			} else {
				ch.Detail = "no delay data (run channel probe)"
			}
		}
		return ch
	}
	ch.DelayMs = last.DelayMs
	ch.Available = last.OK && last.DelayMs > 0
	if typ == "urltest" && urltestMember != "" {
		if snap, ok := engineDelays()[urltestMember]; ok && snap.OK {
			ch.Available = true
		}
	}
	if !ch.Available && last.DelayMs == 0 {
		if !probed {
			if last.Time.IsZero() {
				ch.Detail = "no delay data (run channel probe)"
			} else {
				ch.Detail = "unavailable"
			}
		}
	}
	return ch
}

func channelSelected(name, activeOutbound, urltestMember, urltestGroup string) bool {
	if activeOutbound != "" && name == activeOutbound {
		return true
	}
	if urltestMember == "" || name != urltestMember {
		return false
	}
	// Traffic follows the selector. The urltest winner is "active" only while
	// the selector is the urltest group (or already that member).
	if activeOutbound == "" || activeOutbound == urltestGroup || activeOutbound == urltestMember {
		return true
	}
	return false
}

func nativeDelayCache() map[string]delayhistory.Sample {
	out := make(map[string]delayhistory.Sample)
	rows, err := delayhistory.ReadAll()
	if err != nil {
		return mergeEngineDelayCache(out)
	}
	for _, row := range rows {
		if len(row.Samples) == 0 {
			continue
		}
		out[row.Channel] = row.Samples[len(row.Samples)-1]
	}
	return mergeEngineDelayCache(out)
}

func engineDelays() map[string]engine.DelayChannelState {
	return engine.DelaysFromSnapshot()
}

func mergeEngineDelayCache(base map[string]delayhistory.Sample) map[string]delayhistory.Sample {
	for tag, st := range engineDelays() {
		if _, ok := base[tag]; ok {
			continue
		}
		base[tag] = delayhistory.Sample{
			DelayMs: st.DelayMs,
			OK:      st.OK,
			Time:    time.Now().UTC(),
		}
	}
	return base
}

func proxyScheme(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "://"); i > 0 {
		return raw[:i]
	}
	return "proxy"
}

// skipDuplicateAWG2Link returns true when link is awg2:// with a public_key already seen.
// Alternate Host:Port of the same peer share one outbound (endpoint rotation).
func skipDuplicateAWG2Link(seen map[string]struct{}, link string) bool {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "vpn://") {
		decoded, err := amnezia.DecodeVPNURI(link)
		if err != nil {
			return false
		}
		link = decoded
	}
	if !strings.HasPrefix(link, "awg2://") {
		return false
	}
	params, err := amnezia.ParseAWG2URI(link)
	if err != nil || params.PublicKey == "" {
		return false
	}
	if _, ok := seen[params.PublicKey]; ok {
		return true
	}
	seen[params.PublicKey] = struct{}{}
	return false
}

func hostPortFromURL(rawURL string) string {
	if rawURL == "" {
		return "www.gstatic.com:443"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "www.gstatic.com:443"
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}
