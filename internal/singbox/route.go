package singbox

import (
	"fmt"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

type routeRules struct {
	rules   []map[string]any
	nextTag int
}

func newRouteRules() *routeRules {
	return &routeRules{nextTag: 1}
}

func (r *routeRules) add(rule map[string]any) string {
	tag := fmt.Sprintf("route-rule-%d", r.nextTag)
	r.nextTag++
	rule["__route_rule__"] = tag
	r.rules = append(r.rules, rule)
	return tag
}

func (r *routeRules) patch(tag, key string, value any) {
	for i, rule := range r.rules {
		if rule["__route_rule__"] != tag {
			continue
		}
		if existing, ok := rule[key]; ok && key == "rule_set" {
			switch cur := existing.(type) {
			case string:
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings([]string{cur}, v)
				}
			case []string:
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings(cur, v)
				}
			case []any:
				var ss []string
				for _, item := range cur {
					if s, ok := item.(string); ok {
						ss = append(ss, s)
					}
				}
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings(ss, v)
				}
			}
		} else if existing, ok := rule[key]; ok && key == "source_ip_cidr" {
			switch cur := existing.(type) {
			case string:
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings([]string{cur}, v)
				}
			case []string:
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings(cur, v)
				}
			case []any:
				var ss []string
				for _, item := range cur {
					if s, ok := item.(string); ok {
						ss = append(ss, s)
					}
				}
				if v, ok := value.(string); ok {
					rule[key] = appendUniqueStrings(ss, v)
				}
			}
		} else {
			rule[key] = value
		}
		r.rules[i] = rule
		return
	}
}

func (b *Builder) configureRoute() {
	rr := newRouteRules()

	outputIface := b.outputNetworkInterface()
	autoDetect := outputIface == ""
	route := map[string]any{
		"final":                   DirectTag,
		"auto_detect_interface":   autoDetect,
		"default_domain_resolver": DNSServerTag,
	}
	if outputIface != "" {
		route["default_interface"] = outputIface
	}

	rr.add(map[string]any{
		"action":  "sniff",
		"inbound": []string{TPROXYInboundTag, DNSInboundTag},
	})
	rr.add(map[string]any{
		"action":   "hijack-dns",
		"protocol": "dns",
	})

	settings := b.pkg.Section("settings")
	if settings != nil && settings.GetBool("disable_quic", false) {
		tag := rr.add(map[string]any{
			"action":  "reject",
			"inbound": TPROXYInboundTag,
		})
		rr.patch(tag, "protocol", "quic")
	}

	firstSection := b.firstOutboundSection()
	if firstSection != "" {
		tag := rr.add(map[string]any{
			"action":   "route",
			"inbound":  TPROXYInboundTag,
			"outbound": OutboundTag(firstSection),
		})
		rr.patch(tag, "domain", CheckProxyIPDomain)
	}

	tag := rr.add(map[string]any{
		"action": "route-options",
	})
	rr.patch(tag, "domain", FAKEIPTestDomain)
	rr.patch(tag, "override_port", FakeIPTestPort)

	b.configureFullyRoutedIPs(rr)
	b.configureBlockRejectRule(rr)
	b.configureRoutingExcludedIPs(rr)
	b.configureSectionListRouting(rr)
	b.configureCatchAllSections(rr)

	route["rules"] = rr.rules
	if len(b.cfg.RuleSets) > 0 {
		route["rule_set"] = b.cfg.RuleSets
	}
	b.cfg.Route = route
}

func (b *Builder) configureFullyRoutedIPs(rr *routeRules) {
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if sec == nil {
			continue
		}
		ips := sec.GetList("fully_routed_ips")
		if len(ips) == 0 {
			continue
		}
		tag := rr.add(map[string]any{
			"action":   "route",
			"inbound":  TPROXYInboundTag,
			"outbound": OutboundTag(name),
		})
		for _, ip := range ips {
			rr.patch(tag, "source_ip_cidr", ip)
		}
	}
}

func (b *Builder) configureBlockRejectRule(rr *routeRules) {
	blockWithLists := false
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if sec == nil || sec.Get("connection_type", "") != "block" {
			continue
		}
		if sectionHasEnabledLists(sec) {
			blockWithLists = true
			break
		}
	}
	if !blockWithLists {
		return
	}
	rr.rules = append(rr.rules, map[string]any{
		"action":         "reject",
		"inbound":        TPROXYInboundTag,
		"__route_rule__": RejectRuleTag,
	})
}

func (b *Builder) configureRoutingExcludedIPs(rr *routeRules) {
	settings := b.pkg.Section("settings")
	if settings == nil {
		return
	}
	ips := settings.GetList("routing_excluded_ips")
	if len(ips) == 0 {
		return
	}
	tag := rr.add(map[string]any{
		"action":   "route",
		"inbound":  TPROXYInboundTag,
		"outbound": DirectTag,
	})
	for _, ip := range ips {
		rr.patch(tag, "source_ip_cidr", ip)
	}
}

func (b *Builder) configureCatchAllSections(rr *routeRules) {
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if sec == nil {
			continue
		}
		conn := sec.Get("connection_type", "")
		if conn != "vpn" && conn != "proxy" {
			continue
		}
		if sectionHasEnabledLists(sec) {
			continue
		}
		rr.add(map[string]any{
			"action":   "route",
			"inbound":  TPROXYInboundTag,
			"outbound": OutboundTag(name),
		})
	}
}

func (b *Builder) firstOutboundSection() string {
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if sec == nil {
			continue
		}
		conn := sec.Get("connection_type", "")
		if conn == "proxy" || conn == "vpn" {
			return name
		}
	}
	return ""
}

func sectionHasEnabledLists(sec *uci.Section) bool {
	return SectionHasEnabledLists(sec)
}

// SectionHasEnabledLists reports whether a routing section uses any list-based rules.
func SectionHasEnabledLists(sec *uci.Section) bool {
	if sec == nil {
		return false
	}
	if len(sec.GetList("community_lists")) > 0 {
		return true
	}
	if sec.Get("user_domain_list_type", "disabled") != "disabled" {
		return true
	}
	if sec.Get("user_subnet_list_type", "disabled") != "disabled" {
		return true
	}
	if len(sec.GetList("local_domain_lists")) > 0 {
		return true
	}
	if len(sec.GetList("local_subnet_lists")) > 0 {
		return true
	}
	if len(sec.GetList("remote_domain_lists")) > 0 {
		return true
	}
	if len(sec.GetList("remote_subnet_lists")) > 0 {
		return true
	}
	return false
}
