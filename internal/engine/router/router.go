package router

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/control"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/outbound"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/engine/plan"
)

type Router struct {
	mu            sync.RWMutex
	plan          *plan.Plan
	registry      *outbound.Registry
	control       *control.Control
	rules         []compiledRule
	fakeIP        *fakeIPStore
	domainSetsMap map[string][]string
	subnetSets    map[string][]*net.IPNet
	// downDirect: sections whose traffic may go direct when their channels
	// fail (direct_when_down), with the channel tags to watch.
	downDirect map[string][]string
	// bindingSection maps a list_route to its section and on_down.
	bindingSection map[string]plan.BindingPlan
}

// route is where a connection goes, and the section whose direct_when_down
// may take over when that fails.
type route struct {
	tag     string
	section string
}

type compiledRule struct {
	rule         plan.RouteRule
	domainSuffix []string
	subnets      []*net.IPNet
	sourceNets   []*net.IPNet
	catchAll     bool
}

func New(p *plan.Plan, reg *outbound.Registry, ctrl *control.Control) (*Router, error) {
	r := &Router{
		plan:     p,
		registry: reg,
		control:  ctrl,
		fakeIP:   newFakeIPStore(p.DNS.FakeIPRange),
	}
	r.downDirect = make(map[string][]string)
	for _, sec := range p.Sections {
		if sec.DirectWhenDown {
			r.downDirect[sec.Name] = []string{}
		}
	}
	for _, ch := range p.Channels {
		if tags, ok := r.downDirect[ch.Section]; ok {
			r.downDirect[ch.Section] = append(tags, ch.Tag)
		}
	}
	r.bindingSection = make(map[string]plan.BindingPlan, len(p.Bindings))
	for _, b := range p.Bindings {
		r.bindingSection[b.Name] = b
	}
	for _, rs := range p.RuleSets {
		if err := r.loadRuleSet(rs); err != nil {
			return nil, err
		}
	}
	for _, rule := range p.Routes {
		cr := compiledRule{rule: rule}
		for _, tag := range rule.RuleSetTags {
			if suffixes, ok := r.domainSetsMap[tag]; ok {
				cr.domainSuffix = append(cr.domainSuffix, suffixes...)
			}
			if nets, ok := r.subnetSets[tag]; ok {
				cr.subnets = append(cr.subnets, nets...)
			}
		}
		for _, sfx := range rule.DomainSuffix {
			cr.domainSuffix = append(cr.domainSuffix, sfx)
		}
		for _, d := range rule.Domains {
			cr.domainSuffix = append(cr.domainSuffix, d)
		}
		for _, cidr := range rule.IPCIDR {
			if n, err := parseCIDR(cidr); err == nil {
				cr.subnets = append(cr.subnets, n)
			}
		}
		for _, cidr := range rule.SourceIPCIDR {
			if n, err := parseCIDR(cidr); err == nil {
				cr.sourceNets = append(cr.sourceNets, n)
			}
		}
		cr.catchAll = len(cr.domainSuffix) == 0 && len(cr.subnets) == 0 && len(cr.sourceNets) == 0 && !rule.Reject
		r.rules = append(r.rules, cr)
	}
	return r, nil
}

func (r *Router) Route(meta plan.ConnMeta) (string, error) {
	rt, err := r.route(meta)
	return rt.tag, err
}

// route picks the outbound for a connection. While every channel of the
// section is down, a section with direct_when_down goes direct right away
// instead of failing each connection on a dead tunnel.
func (r *Router) route(meta plan.ConnMeta) (route, error) {
	rt, err := r.match(meta)
	if err != nil || rt.section == "" || rt.tag == plan.DirectTag {
		return rt, err
	}
	if r.sectionDown(rt.section) {
		return route{tag: plan.DirectTag}, nil
	}
	return rt, nil
}

func (r *Router) match(meta plan.ConnMeta) (route, error) {
	domain := meta.Domain
	if domain == "" && meta.DstIP != "" {
		if d, ok := r.fakeIP.LookupDomain(meta.DstIP); ok {
			domain = d
		}
	}
	dstIP := net.ParseIP(meta.DstIP)
	srcIP := net.ParseIP(meta.SrcIP)
	for _, cr := range r.rules {
		if cr.rule.Network != "" && meta.Network != "" && cr.rule.Network != meta.Network {
			continue
		}
		if cr.rule.Reject {
			if ruleMatches(cr, domain, dstIP, srcIP) {
				return route{}, fmt.Errorf("rejected")
			}
			continue
		}
		if !ruleMatches(cr, domain, dstIP, srcIP) {
			continue
		}
		tag, err := r.resolveOutbound(cr.rule.Section, cr.rule.OutboundTag)
		return route{tag: tag, section: r.directSection(cr.rule)}, err
	}
	// List-based sections: sing-box final=direct. Do not catch-all to VPN/proxy
	// or Russian sites (Yandex Music, etc.) leak into the tunnel via tproxy.
	for _, sec := range r.plan.Sections {
		if sec.ConnectionType != "vpn" && sec.ConnectionType != "proxy" {
			continue
		}
		if sec.ListBased {
			continue
		}
		tag, err := r.resolveOutbound(sec.Name, plan.OutboundTag(sec.Name))
		section := ""
		if _, ok := r.downDirect[sec.Name]; ok {
			section = sec.Name
		}
		return route{tag: tag, section: section}, err
	}
	return route{tag: plan.DirectTag}, nil
}

// directSection returns the section whose direct_when_down covers rule, or
// "" when the rule must not go direct: a section without the option, or a
// list bound with on_down block (the user chose blocked over direct).
func (r *Router) directSection(rule plan.RouteRule) string {
	section := rule.Section
	if rule.Binding != "" {
		b, ok := r.bindingSection[rule.Binding]
		if !ok || b.OnDown == "block" {
			return ""
		}
		section = b.Section
	}
	if _, ok := r.downDirect[section]; !ok {
		return ""
	}
	return section
}

// sectionDown reports whether no channel of section works: each one failed
// its last probe. A channel never probed yet counts as alive. A section with
// no channels (compiled direct) is handled by its own direct outbound.
func (r *Router) sectionDown(section string) bool {
	tags := r.downDirect[section]
	if r.control == nil || len(tags) == 0 {
		return false
	}
	for _, tag := range tags {
		d := r.control.Delay(tag)
		if d.OK || d.Error == "" {
			return false
		}
	}
	return true
}

func ruleMatches(cr compiledRule, domain string, dstIP, srcIP net.IP) bool {
	if cr.catchAll {
		return true
	}
	if len(cr.sourceNets) > 0 {
		if srcIP == nil || !ipInNets(srcIP, cr.sourceNets) {
			return false
		}
		// Source-only full_route: any destination for this LAN client.
		if len(cr.domainSuffix) == 0 && len(cr.subnets) == 0 {
			return true
		}
	}
	if domain != "" {
		for _, sfx := range cr.domainSuffix {
			if domainMatches(domain, sfx) {
				return true
			}
		}
	}
	if dstIP != nil {
		for _, n := range cr.subnets {
			if n.Contains(dstIP) {
				return true
			}
		}
	}
	return false
}

func ipInNets(ip net.IP, nets []*net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func parseCIDR(raw string) (*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty cidr")
	}
	if !strings.Contains(raw, "/") {
		ip := net.ParseIP(raw)
		if ip == nil {
			return nil, fmt.Errorf("bad ip")
		}
		if v4 := ip.To4(); v4 != nil {
			return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}, nil
		}
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}, nil
	}
	_, n, err := net.ParseCIDR(raw)
	return n, err
}

func (r *Router) resolveOutbound(section, fallback string) (string, error) {
	if r.control != nil && section != "" {
		return r.control.ActiveTag(section), nil
	}
	return fallback, nil
}

// DialUpstream connects to the DNS upstream through the active channel of
// the first vpn or proxy section, the one Route falls back to. The DNS
// server races it against plain UDP from the WAN, so a provider that drops
// UDP/53 does not take every unlisted site down with it.
func (r *Router) DialUpstream(ctx context.Context, addr string) (net.Conn, error) {
	for _, sec := range r.plan.Sections {
		if sec.ConnectionType != "vpn" && sec.ConnectionType != "proxy" {
			continue
		}
		tag, err := r.resolveOutbound(sec.Name, plan.OutboundTag(sec.Name))
		if err != nil {
			return nil, err
		}
		return r.registry.DialTCP(ctx, tag, "tcp", addr)
	}
	return nil, fmt.Errorf("no vpn or proxy section to carry the dns upstream")
}

// Dial connects through the routed outbound. When that fails and the section
// has direct_when_down, it tries direct: a tunnel can die between two probes,
// and the client should not wait for the next round to get online.
func (r *Router) Dial(meta plan.ConnMeta) (net.Conn, error) {
	rt, err := r.route(meta)
	if err != nil {
		return nil, err
	}
	target := r.dialTarget(meta)
	c, err := r.registry.DialTCP(context.Background(), rt.tag, "tcp", target)
	if err != nil && rt.section != "" {
		if dc, derr := r.registry.DialTCP(context.Background(), plan.DirectTag, "tcp", target); derr == nil {
			return dc, nil
		}
	}
	return c, err
}

func (r *Router) DialUDP(ctx context.Context, meta plan.ConnMeta) (net.PacketConn, error) {
	rt, err := r.route(meta)
	if err != nil {
		return nil, err
	}
	target := r.dialTarget(meta)
	c, err := r.registry.DialUDP(ctx, rt.tag, "udp", target)
	if err != nil && rt.section != "" && ctx.Err() == nil {
		if dc, derr := r.registry.DialUDP(ctx, plan.DirectTag, "udp", target); derr == nil {
			return dc, nil
		}
	}
	return c, err
}

// dialTarget returns the hostname proxy outbounds should connect to.
// FakeIP addresses are mapped back to the original domain before dialing.
func (r *Router) dialTarget(meta plan.ConnMeta) string {
	host := strings.TrimSpace(meta.DstIP)
	if meta.Domain != "" {
		host = meta.Domain
	} else if d, ok := r.fakeIP.LookupDomain(meta.DstIP); ok {
		host = d
	}
	return net.JoinHostPort(host, fmtPort(meta.DstPort))
}

func (r *Router) ResolveFakeIP(domain string) (string, bool) {
	return r.fakeIP.Allocate(domain)
}

// ShouldFakeIP reports whether DNS should return a fakeip address for domain.
func (r *Router) ShouldFakeIP(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false
	}
	for _, d := range []string{plan.FakeIPTestDomain, plan.CheckProxyIPDomain} {
		if domain == strings.ToLower(d) {
			return true
		}
	}
	for _, suffixes := range r.domainSetsMap {
		for _, sfx := range suffixes {
			if domainMatches(domain, sfx) {
				return true
			}
		}
	}
	for _, cr := range r.rules {
		for _, sfx := range cr.domainSuffix {
			if domainMatches(domain, sfx) {
				return true
			}
		}
	}
	return false
}

func domainMatches(domain, suffix string) bool {
	suffix = strings.TrimPrefix(suffix, ".")
	domain = strings.ToLower(domain)
	suffix = strings.ToLower(suffix)
	return domain == suffix || strings.HasSuffix(domain, "."+suffix)
}

func fmtPort(p int) string {
	return fmt.Sprintf("%d", p)
}
