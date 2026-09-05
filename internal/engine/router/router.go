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
	domain := meta.Domain
	if domain == "" && meta.DstIP != "" {
		if d, ok := r.fakeIP.LookupDomain(meta.DstIP); ok {
			domain = d
		}
	}
	dstIP := net.ParseIP(meta.DstIP)
	srcIP := net.ParseIP(meta.SrcIP)
	for _, cr := range r.rules {
		if cr.rule.Reject {
			if ruleMatches(cr, domain, dstIP, srcIP) {
				return "", fmt.Errorf("rejected")
			}
			continue
		}
		if !ruleMatches(cr, domain, dstIP, srcIP) {
			continue
		}
		return r.resolveOutbound(cr.rule.Section, cr.rule.OutboundTag)
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
		return r.resolveOutbound(sec.Name, plan.OutboundTag(sec.Name))
	}
	return plan.DirectTag, nil
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

func (r *Router) Dial(meta plan.ConnMeta) (net.Conn, error) {
	tag, err := r.Route(meta)
	if err != nil {
		return nil, err
	}
	return r.registry.DialTCP(context.Background(), tag, "tcp", r.dialTarget(meta))
}

func (r *Router) DialUDP(ctx context.Context, meta plan.ConnMeta) (net.PacketConn, error) {
	tag, err := r.Route(meta)
	if err != nil {
		return nil, err
	}
	return r.registry.DialUDP(ctx, tag, "udp", r.dialTarget(meta))
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
