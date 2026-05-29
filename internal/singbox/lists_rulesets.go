package singbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func (b *Builder) configureSectionListRouting(rr *routeRules) {
	for _, name := range b.pkg.SectionNames("section") {
		sec := b.pkg.Section(name)
		if sec == nil || !sectionHasEnabledLists(sec) {
			continue
		}
		b.configureRoutingForSectionLists(name, sec, rr)
	}
}

func (b *Builder) configureRoutingForSectionLists(section string, sec *uci.Section, rr *routeRules) {
	conn := sec.Get("connection_type", "")
	var routeRuleTag string
	if conn == "block" {
		routeRuleTag = RejectRuleTag
	} else {
		routeRuleTag = rr.add(map[string]any{
			"action":   "route",
			"inbound":  TPROXYInboundTag,
			"outbound": OutboundTag(section),
		})
	}

	for _, tag := range sec.GetList("community_lists") {
		b.addCommunityRuleset(section, tag, routeRuleTag, rr)
	}

	if sec.Get("user_domain_list_type", "disabled") != "disabled" {
		b.addUserDomainRuleset(section, sec, routeRuleTag, rr)
	}
	if sec.Get("user_subnet_list_type", "disabled") != "disabled" {
		b.addUserSubnetRuleset(section, sec, routeRuleTag, rr)
	}

	for _, path := range sec.GetList("local_domain_lists") {
		b.addLocalRuleset(section, "local", "domains", path, routeRuleTag, rr)
	}
	for _, path := range sec.GetList("local_subnet_lists") {
		b.addLocalRuleset(section, "local", "subnets", path, routeRuleTag, rr)
	}
	for _, url := range sec.GetList("remote_domain_lists") {
		b.addRemoteListRuleset(section, url, "domains", routeRuleTag, rr)
	}
	for _, url := range sec.GetList("remote_subnet_lists") {
		b.addRemoteListRuleset(section, url, "subnets", routeRuleTag, rr)
	}
}

func (b *Builder) addCommunityRuleset(section, service, routeRuleTag string, rr *routeRules) {
	tag := RulesetTag(section, service, "community")
	rs := map[string]any{
		"type":            "remote",
		"tag":             tag,
		"format":          "binary",
		"url":             CommunitySRSURL(service),
		"update_interval": b.updateInterval(),
	}
	if detour := b.downloadDetourTag(); detour != "" {
		rs["download_detour"] = detour
	}
	b.cfg.AddRuleSet(rs)
	rr.patch(routeRuleTag, "rule_set", tag)
	b.patchFakeIPDNSRule("rule_set", tag)
}

func (b *Builder) addLocalRuleset(section, name, typ, listPath, routeRuleTag string, rr *routeRules) {
	tag := RulesetTag(section, name, typ)
	path := listPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(RulesetDir, filepath.Base(path))
	}
	b.ensureSourceRuleset(path)
	b.cfg.AddRuleSet(map[string]any{
		"type":   "local",
		"tag":    tag,
		"format": "source",
		"path":   path,
	})
	rr.patch(routeRuleTag, "rule_set", tag)
	if typ == "domains" {
		b.patchFakeIPDNSRule("rule_set", tag)
	}
}

func (b *Builder) addRemoteListRuleset(section, rawURL, typ, routeRuleTag string, rr *routeRules) {
	ext := fileExtension(rawURL)
	switch ext {
	case "json", "srs":
		tag := RulesetTag(section, fileBaseName(rawURL), "remote-"+typ)
		rs := map[string]any{
			"type":            "remote",
			"tag":             tag,
			"format":          rulesetFormat(ext),
			"url":             rawURL,
			"update_interval": b.updateInterval(),
		}
		if detour := b.downloadDetourTag(); detour != "" {
			rs["download_detour"] = detour
		}
		b.cfg.AddRuleSet(rs)
		rr.patch(routeRuleTag, "rule_set", tag)
		if typ == "domains" {
			b.patchFakeIPDNSRule("rule_set", tag)
		}
	default:
		tag := RulesetTag(section, "remote", typ)
		path := filepath.Join(RulesetDir, tag+".json")
		b.ensureSourceRuleset(path)
		b.cfg.AddRuleSet(map[string]any{
			"type":   "local",
			"tag":    tag,
			"format": "source",
			"path":   path,
		})
		rr.patch(routeRuleTag, "rule_set", tag)
		if typ == "domains" {
			b.patchFakeIPDNSRule("rule_set", tag)
		}
	}
}

func (b *Builder) addUserDomainRuleset(section string, sec *uci.Section, routeRuleTag string, rr *routeRules) {
	tag := RulesetTag(section, "user", "domains")
	path := filepath.Join(RulesetDir, tag+".json")
	domains := b.userDomainItems(sec)
	if err := writeDomainRuleset(path, domains); err != nil {
		return
	}
	b.cfg.AddRuleSet(map[string]any{
		"type":   "local",
		"tag":    tag,
		"format": "source",
		"path":   path,
	})
	rr.patch(routeRuleTag, "rule_set", tag)
	b.patchFakeIPDNSRule("rule_set", tag)
}

func (b *Builder) addUserSubnetRuleset(section string, sec *uci.Section, routeRuleTag string, rr *routeRules) {
	tag := RulesetTag(section, "user", "subnets")
	path := filepath.Join(RulesetDir, tag+".json")
	cidrs := b.userSubnetItems(sec)
	if err := writeSubnetRuleset(path, cidrs); err != nil {
		return
	}
	b.cfg.AddRuleSet(map[string]any{
		"type":   "local",
		"tag":    tag,
		"format": "source",
		"path":   path,
	})
	rr.patch(routeRuleTag, "rule_set", tag)
}

func (b *Builder) userDomainItems(sec *uci.Section) []string {
	switch sec.Get("user_domain_list_type", "disabled") {
	case "dynamic":
		return sec.GetList("user_domains")
	case "text":
		return splitListItems(sec.Get("user_domains_text", ""))
	default:
		return nil
	}
}

func (b *Builder) userSubnetItems(sec *uci.Section) []string {
	switch sec.Get("user_subnet_list_type", "disabled") {
	case "dynamic":
		return sec.GetList("user_subnets")
	case "text":
		return splitListItems(sec.Get("user_subnets_text", ""))
	default:
		return nil
	}
}

func splitListItems(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == ' '
	})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (b *Builder) updateInterval() string {
	settings := b.pkg.Section("settings")
	if settings == nil {
		return DefaultUpdateInterval
	}
	if v := settings.Get("update_interval", ""); v != "" {
		return v
	}
	return DefaultUpdateInterval
}

func (b *Builder) downloadDetourTag() string {
	settings := b.pkg.Section("settings")
	if settings == nil || !settings.GetBool("download_lists_via_proxy", false) {
		return ""
	}
	section := settings.Get("download_lists_via_proxy_section", "")
	if section == "" {
		return ""
	}
	return OutboundTag(section)
}

func (b *Builder) ensureSourceRuleset(path string) {
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(`{"version":3,"rules":[]}`+"\n"), 0o644)
}

func writeDomainRuleset(path string, domains []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(domains) == 0 {
		return os.WriteFile(path, []byte(`{"version":3,"rules":[]}`+"\n"), 0o644)
	}
	body, err := json.Marshal(map[string]any{
		"version": 3,
		"rules": []map[string]any{
			{"domain_suffix": domains},
		},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func writeSubnetRuleset(path string, cidrs []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if len(cidrs) == 0 {
		return os.WriteFile(path, []byte(`{"version":3,"rules":[]}`+"\n"), 0o644)
	}
	body, err := json.Marshal(map[string]any{
		"version": 3,
		"rules": []map[string]any{
			{"ip_cidr": cidrs},
		},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func sanitizeInternalTags(cfg *Config) {
	if rules, ok := cfg.DNS["rules"].([]map[string]any); ok {
		for i, rule := range rules {
			delete(rule, "__dns_rule__")
			rules[i] = rule
		}
		cfg.DNS["rules"] = rules
	}
	if rules, ok := cfg.Route["rules"].([]map[string]any); ok {
		for i, rule := range rules {
			delete(rule, "__route_rule__")
			rules[i] = rule
		}
		cfg.Route["rules"] = rules
	}
}
