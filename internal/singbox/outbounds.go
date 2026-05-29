package singbox

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/policy"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

func (b *Builder) configureOutbounds() error {
	b.cfg.AddOutbound(map[string]any{
		"type": "direct",
		"tag":  DirectTag,
	})
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if err := b.configureSectionOutbound(name, sec); err != nil {
			return fmt.Errorf("section %q: %w", name, err)
		}
	}
	return nil
}

func (b *Builder) configureSectionOutbound(section string, sec *uci.Section) error {
	connType := sec.Get("connection_type", "")
	switch connType {
	case "proxy":
		return b.configureProxy(section, sec)
	case "vpn":
		return b.configureVPN(section, sec)
	case "block":
		return nil
	default:
		if connType == "" {
			return nil
		}
		return fmt.Errorf("unknown connection_type %q", connType)
	}
}

func (b *Builder) configureProxy(section string, sec *uci.Section) error {
	proxyType := sec.Get("proxy_config_type", "url")
	udpOverTCP := sec.GetBool("enable_udp_over_tcp", false)
	switch proxyType {
	case "url":
		link := sec.Get("proxy_string", "")
		if link == "" {
			return fmt.Errorf("proxy_string is not set")
		}
		return b.addProxyLink(section, link, udpOverTCP)
	case "urltest":
		links := sec.GetList("urltest_proxy_links")
		if len(links) == 0 {
			return fmt.Errorf("urltest_proxy_links is not set")
		}
		return b.addURLTestGroup(section, links, sec, udpOverTCP, OutboundTag(section))
	case "outbound":
		raw := sec.Get("outbound_json", "")
		if raw == "" {
			return fmt.Errorf("outbound_json is not set")
		}
		return b.addJSONOutbound(section, raw)
	default:
		return fmt.Errorf("unknown proxy_config_type %q", proxyType)
	}
}

func (b *Builder) configureVPN(section string, sec *uci.Section) error {
	iface := sec.Get("interface", "")
	if iface == "" {
		return fmt.Errorf("interface is not set")
	}
	domainResolverTag := b.queueSectionDomainResolver(section, sec, OutboundTag(section))

	failover := sec.GetBool("failover_vpn_enabled", false)
	links := sec.GetList("failover_proxy_links")
	if !failover || len(links) == 0 {
		tag := OutboundTag(section)
		b.cfg.AddOutbound(vpnDirectOutbound(tag, iface, domainResolverTag))
		return nil
	}
	awgTag := AWGTag(section)
	b.cfg.AddOutbound(vpnDirectOutbound(awgTag, iface, domainResolverTag))
	udpOverTCP := sec.GetBool("enable_udp_over_tcp", false)
	var candidates []string
	candidates = append(candidates, awgTag)
	for i, link := range links {
		peerSection := fmt.Sprintf("%s-%d", section, i+1)
		if err := b.addProxyLink(peerSection, link, udpOverTCP); err != nil {
			return err
		}
		candidates = append(candidates, PeerTag(section, i+1))
	}
	pol := policy.Normalize(sec.Get("failover_policy", ""))
	if pol == policy.Fastest {
		ut := URLTestTag(section)
		return b.addURLTestFromCandidates(section, sec, candidates, ut, OutboundTag(section), ut)
	}
	return b.addManagedVPNFailover(section, sec, awgTag, candidates[1:], OutboundTag(section))
}

func (b *Builder) addJSONOutbound(section, raw string) error {
	tag := OutboundTag(section)
	if err := CheckOutboundJSON(raw, tag); err != nil {
		return fmt.Errorf("outbound_json: %w", err)
	}
	var ob map[string]any
	if err := json.Unmarshal([]byte(raw), &ob); err != nil {
		return fmt.Errorf("outbound_json: %w", err)
	}
	ob["tag"] = tag
	b.cfg.AddOutbound(ob)
	return nil
}

func vpnDirectOutbound(tag, iface, domainResolverTag string) map[string]any {
	ob := map[string]any{
		"type":           "direct",
		"tag":            tag,
		"bind_interface": iface,
	}
	if domainResolverTag != "" {
		ob["domain_resolver"] = domainResolverTag
	}
	return ob
}

func (b *Builder) queueSectionDomainResolver(section string, sec *uci.Section, detour string) string {
	if !sec.GetBool("domain_resolver_enabled", false) {
		return ""
	}
	dnsServer := sec.Get("domain_resolver_dns_server", "")
	if dnsServer == "" {
		return ""
	}
	dnsType := sec.Get("domain_resolver_dns_type", "udp")
	tag := DomainResolverTag(section)
	domainResolver := ""
	if !isIPv4(urlHost(dnsServer)) {
		domainResolver = BootstrapTag
	}
	b.sectionDomainResolvers = append(b.sectionDomainResolvers, sectionDomainResolver{
		tag:            tag,
		dnsType:        dnsType,
		dnsServer:      dnsServer,
		domainResolver: domainResolver,
		detour:         detour,
	})
	return tag
}

func (b *Builder) addProxyLink(section, link string, udpOverTCP bool) error {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "vpn://") {
		decoded, err := amnezia.DecodeVPNURI(link)
		if err != nil {
			return err
		}
		link = decoded
	}
	if strings.HasPrefix(link, "awg2://") {
		ifname := amnezia.AWG2InterfaceName(section)
		b.cfg.AddOutbound(vpnDirectOutbound(OutboundTag(section), ifname, ""))
		return nil
	}
	tag := OutboundTag(section)
	ob, err := uri.ParseProxy(link, tag, udpOverTCP)
	if err != nil {
		return err
	}
	b.cfg.AddOutbound(ob.Fields)
	return nil
}

func (b *Builder) addURLTestGroup(section string, links []string, sec *uci.Section, udpOverTCP bool, selectorTag string) error {
	var candidates []string
	for i, link := range links {
		peerSection := fmt.Sprintf("%s-%d", section, i+1)
		if err := b.addProxyLink(peerSection, link, udpOverTCP); err != nil {
			return err
		}
		candidates = append(candidates, PeerTag(section, i+1))
	}
	return b.addURLTestFromCandidates(section, sec, candidates, URLTestTag(section), selectorTag, selectorTag)
}

// addManagedVPNFailover pins primary VPN on the selector; urltest runs only on backup outbounds.
func (b *Builder) addManagedVPNFailover(section string, sec *uci.Section, primaryTag string, backupTags []string, selectorTag string) error {
	if len(backupTags) == 0 {
		b.cfg.AddOutbound(map[string]any{
			"type":      "selector",
			"tag":       selectorTag,
			"outbounds": []string{primaryTag},
			"default":   primaryTag,
		})
		return nil
	}
	urltestTag := URLTestTag(section)
	if err := b.addURLTestFromCandidates(section, sec, backupTags, urltestTag, "", ""); err != nil {
		return err
	}
	selectorOutbounds := []string{primaryTag, urltestTag}
	selectorOutbounds = append(selectorOutbounds, backupTags...)
	b.cfg.AddOutbound(map[string]any{
		"type":      "selector",
		"tag":       selectorTag,
		"outbounds": selectorOutbounds,
		"default":   primaryTag,
	})
	return nil
}

func (b *Builder) addURLTestFromCandidates(section string, sec *uci.Section, candidates []string, urltestTag, selectorTag, selectorDefault string) error {
	if urltestTag == "" {
		urltestTag = URLTestTag(section)
	}
	checkInterval := sec.Get("urltest_check_interval", "3m")
	idleTimeout := NormalizeDuration(sec.Get("urltest_idle_timeout", ""))
	if err := validation.ValidateURLTestDurationPair(checkInterval, idleTimeout); err != nil {
		return err
	}
	tolerance := sec.Get("urltest_tolerance", "50")
	testingURL := sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204")

	ut := map[string]any{
		"type":      "urltest",
		"tag":       urltestTag,
		"outbounds": candidates,
		"url":       testingURL,
		"interval":  checkInterval,
		"tolerance": parseIntDefault(tolerance, 50),
	}
	if idleTimeout != "" {
		ut["idle_timeout"] = idleTimeout
	}
	if sec.GetBool("urltest_interrupt_exist_connections", false) {
		ut["interrupt_exist_connections"] = true
	}
	b.cfg.AddOutbound(ut)

	if selectorTag == "" {
		return nil
	}
	if selectorDefault == "" {
		selectorDefault = urltestTag
	}
	selectorOutbounds := append(append([]string{}, candidates...), urltestTag)
	b.cfg.AddOutbound(map[string]any{
		"type":      "selector",
		"tag":       selectorTag,
		"outbounds": selectorOutbounds,
		"default":   selectorDefault,
	})
	return nil
}
