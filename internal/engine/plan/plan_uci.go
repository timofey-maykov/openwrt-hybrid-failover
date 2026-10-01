package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/channels"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/clientrules"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/listroutes"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/policy"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/singbox"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

// CompilePlan builds a native engine plan from UCI.
func CompilePlan(pkg *uci.Package) (*Plan, error) {
	if pkg == nil {
		return nil, fmt.Errorf("nil uci package")
	}
	if len(pkg.SectionNames("section")) == 0 {
		return nil, fmt.Errorf("no routing section in UCI")
	}
	c := &compiler{pkg: pkg, plan: &Plan{}}
	if err := c.compileSettings(); err != nil {
		return nil, err
	}
	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		if err := c.compileSection(name, sec); err != nil {
			return nil, fmt.Errorf("section %q: %w", name, err)
		}
	}
	c.compileRoutes()
	c.compileFullyRoutedRoutes()
	c.compileUDPRoutedRoutes()
	return c.plan, nil
}

type compiler struct {
	pkg  *uci.Package
	plan *Plan
	// awg2ByKey maps peer public key → first outbound tag so alternate AWG2
	// Host:Port links of the same peer do not create a second bind iface.
	awg2ByKey map[string]string
}

func (c *compiler) compileSettings() error {
	settings := c.pkg.Section("settings")
	dnsType := singbox.DefaultDNSType
	dnsServer := singbox.DefaultDNSServer
	bootstrap := singbox.DefaultBootstrapDNS
	rewriteTTL := 60
	if settings != nil {
		if v := settings.Get("dns_type", ""); v != "" {
			dnsType = v
		}
		if v := settings.Get("dns_server", ""); v != "" {
			dnsServer = v
		}
		if v := settings.Get("bootstrap_dns_server", ""); v != "" {
			bootstrap = v
		}
		if v := settings.Get("dns_rewrite_ttl", ""); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				rewriteTTL = n
			}
		}
		c.plan.OutputIface = settings.Get("output_network_interface", "")
		if settings.GetBool("download_lists_via_proxy", false) {
			section := settings.Get("download_lists_via_proxy_section", "")
			if section == "" {
				section = settings.Get("main_section", "glob")
			}
			c.plan.ListDownload = ListDownloadPlan{
				Enabled: true,
				Section: section,
				Port:    singbox.ListDownloadMixedPort,
			}
		}
	}
	c.plan.DNS = DNSPlan{
		Type:          dnsType,
		Server:        dnsServer,
		Bootstrap:     bootstrap,
		RewriteTTL:    rewriteTTL,
		FakeIPRange:   FakeIPRange,
		FakeIPDomains: []string{FakeIPTestDomain, CheckProxyIPDomain},
		RejectHTTPS:   true,
	}
	if settings != nil {
		c.plan.DisableQUIC = settings.GetBool("disable_quic", false)
	}
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{Tag: DirectTag, Kind: OutboundDirect})
	return nil
}

func (c *compiler) compileSection(section string, sec *uci.Section) error {
	if !sec.GetBool("enabled", true) {
		return nil
	}
	conn := sec.Get("connection_type", "")
	sp := SectionPlan{Name: section, ConnectionType: conn, Enabled: true}
	switch conn {
	case "vpn":
		if err := c.compileVPN(section, sec); err != nil {
			return err
		}
		sp.SelectorTag = OutboundTag(section)
	case "proxy":
		if err := c.compileProxy(section, sec); err != nil {
			return err
		}
		sp.SelectorTag = OutboundTag(section)
	case "block":
		// lists handled in compileRoutes
	default:
		if conn != "" {
			return fmt.Errorf("unknown connection_type %q", conn)
		}
	}
	if conn == "vpn" || conn == "proxy" {
		c.recordChannels(section, sec)
	}
	if sectionRoutesByLists(c.pkg, section, sec) {
		sp.ListBased = true
		if err := c.compileListRuleSets(section, sec); err != nil {
			return err
		}
	}
	c.plan.Sections = append(c.plan.Sections, sp)
	return nil
}

// sectionRoutesByLists: the section only takes matching traffic (its own lists
// or named user lists) instead of everything that reached tproxy.
func sectionRoutesByLists(pkg *uci.Package, section string, sec *uci.Section) bool {
	return singbox.SectionHasEnabledLists(sec) || listroutes.HasUserLists(pkg, section)
}

// recordChannels maps the stable channel ids of section to engine tags. The
// tag of a link follows its position (section-N-out); alternate IPs of one AWG
// peer share the first one's tag, as channels.List reports them once.
func (c *compiler) recordChannels(section string, sec *uci.Section) {
	conn := sec.Get("connection_type", "")
	for _, ch := range channels.List(sec) {
		tag := ""
		switch {
		case ch.Primary:
			if sec.GetBool("failover_vpn_enabled", false) && len(sec.GetList("failover_proxy_links")) > 0 {
				tag = AWGTag(section)
			} else {
				tag = OutboundTag(section)
			}
		case conn == "proxy" && sec.Get("proxy_config_type", "url") == "url":
			tag = OutboundTag(section)
		default:
			tag = OutboundTag(fmt.Sprintf("%s-%d", section, ch.Index))
			if strings.HasPrefix(ch.Link, "awg2://") {
				if p, err := amnezia.ParseAWG2URI(ch.Link); err == nil {
					if t, ok := c.awg2ByKey[p.PublicKey]; ok {
						tag = t
					}
				}
			}
		}
		if !c.hasOutbound(tag) {
			continue
		}
		c.plan.Channels = append(c.plan.Channels, ChannelPlan{Section: section, ID: ch.ID, Name: ch.Name, Tag: tag})
	}
}

func (c *compiler) hasOutbound(tag string) bool {
	for _, ob := range c.plan.Outbounds {
		if ob.Tag == tag {
			return true
		}
	}
	return false
}

func (c *compiler) channelTags(section string) []string {
	var out []string
	for _, ch := range c.plan.Channels {
		if ch.Section == section {
			out = append(out, ch.Tag)
		}
	}
	return out
}

func (c *compiler) channelTag(section, id string) string {
	for _, ch := range c.plan.Channels {
		if ch.Section == section && ch.ID == id {
			return ch.Tag
		}
	}
	return ""
}

func (c *compiler) compileVPN(section string, sec *uci.Section) error {
	iface := sec.Get("interface", "")
	if iface == "" {
		return fmt.Errorf("interface is not set")
	}
	failover := sec.GetBool("failover_vpn_enabled", false)
	links := sec.GetList("failover_proxy_links")
	if !failover || len(links) == 0 {
		c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
			Tag:       OutboundTag(section),
			Kind:      OutboundDirectBind,
			BindIface: iface,
		})
		return nil
	}
	awgTag := AWGTag(section)
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
		Tag:       awgTag,
		Kind:      OutboundDirectBind,
		BindIface: iface,
	})
	udpOverTCP := sec.GetBool("enable_udp_over_tcp", false)
	var backupTags []string
	for i, link := range links {
		peerSection := fmt.Sprintf("%s-%d", section, i+1)
		tag, err := c.addProxyLink(peerSection, link, udpOverTCP)
		if err != nil {
			return err
		}
		if tag == "" {
			continue
		}
		backupTags = append(backupTags, tag)
	}
	pol := policy.Normalize(sec.Get("failover_policy", ""))
	if pol == policy.Fastest {
		return c.addURLTestGroup(section, sec, append([]string{awgTag}, backupTags...), OutboundTag(section), URLTestTag(section))
	}
	return c.addManagedVPNFailover(section, sec, awgTag, backupTags)
}

func (c *compiler) addJSONOutbound(section, raw string) error {
	var ob map[string]any
	if err := json.Unmarshal([]byte(raw), &ob); err != nil {
		return fmt.Errorf("outbound_json: %w", err)
	}
	typ, _ := ob["type"].(string)
	tag := OutboundTag(section)
	switch typ {
	case "socks":
		server, _ := ob["server"].(string)
		port := 1080
		if v, ok := ob["server_port"].(float64); ok {
			port = int(v)
		}
		c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
			Tag:      tag,
			Kind:     OutboundSocks,
			ProxyURI: fmt.Sprintf("socks5://%s:%d", server, port),
		})
	case "direct":
		bind, _ := ob["bind_interface"].(string)
		c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
			Tag:       tag,
			Kind:      OutboundDirectBind,
			BindIface: bind,
		})
	default:
		return fmt.Errorf("outbound_json type %q not supported in native engine", typ)
	}
	return nil
}

func (c *compiler) compileProxy(section string, sec *uci.Section) error {
	proxyType := sec.Get("proxy_config_type", "url")
	udpOverTCP := sec.GetBool("enable_udp_over_tcp", false)
	switch proxyType {
	case "url":
		link := sec.Get("proxy_string", "")
		if link == "" {
			return fmt.Errorf("proxy_string is not set")
		}
		_, err := c.addProxyLink(section, link, udpOverTCP)
		return err
	case "urltest":
		links := sec.GetList("urltest_proxy_links")
		if len(links) == 0 {
			return fmt.Errorf("urltest_proxy_links is not set")
		}
		var candidates []string
		for i, link := range links {
			peerSection := fmt.Sprintf("%s-%d", section, i+1)
			tag, err := c.addProxyLink(peerSection, link, udpOverTCP)
			if err != nil {
				return err
			}
			if tag == "" {
				continue
			}
			candidates = append(candidates, tag)
		}
		if len(candidates) == 0 {
			return fmt.Errorf("urltest_proxy_links produced no outbounds")
		}
		return c.addURLTestGroup(section, sec, candidates, OutboundTag(section), URLTestTag(section))
	case "outbound":
		raw := sec.Get("outbound_json", "")
		if raw == "" {
			return fmt.Errorf("outbound_json is not set")
		}
		return c.addJSONOutbound(section, raw)
	default:
		return fmt.Errorf("unknown proxy_config_type %q", proxyType)
	}
}

func (c *compiler) addProxyLink(section, link string, udpOverTCP bool) (string, error) {
	link = strings.TrimSpace(link)
	if strings.HasPrefix(link, "vpn://") {
		decoded, err := amnezia.DecodeVPNURI(link)
		if err != nil {
			return "", err
		}
		link = decoded
	}
	if strings.HasPrefix(link, "awg2://") {
		params, err := amnezia.ParseAWG2URI(link)
		if err != nil {
			return "", err
		}
		if c.awg2ByKey == nil {
			c.awg2ByKey = make(map[string]string)
		}
		if _, ok := c.awg2ByKey[params.PublicKey]; ok {
			// Same peer, alternate IP: endpoint list is handled by lifecycle setup.
			return "", nil
		}
		tag := OutboundTag(section)
		c.awg2ByKey[params.PublicKey] = tag
		c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
			Tag:       tag,
			Kind:      OutboundAWG2Bind,
			BindIface: amnezia.AWG2InterfaceName(section),
		})
		return tag, nil
	}
	if err := validation.ValidateProxyURI(link); err != nil {
		return "", err
	}
	tag := OutboundTag(section)
	kind := outboundKindFromURI(link)
	ob := OutboundPlan{
		Tag:      tag,
		Kind:     kind,
		ProxyURI: link,
	}
	if proxyNeedsWANBind(kind) {
		ob.BindIface = defaultWANInterface()
	}
	c.plan.Outbounds = append(c.plan.Outbounds, ob)
	_ = udpOverTCP
	return tag, nil
}

func outboundKindFromURI(raw string) OutboundKind {
	u := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(u, "vless://"):
		return OutboundVLESS
	case strings.HasPrefix(u, "trojan://"):
		return OutboundTrojan
	case strings.HasPrefix(u, "ss://"):
		return OutboundShadowsocks
	case strings.HasPrefix(u, "socks"):
		return OutboundSocks
	case strings.HasPrefix(u, "hy2://"), strings.HasPrefix(u, "hysteria2://"):
		return OutboundHysteria2
	default:
		return OutboundVLESS
	}
}

func (c *compiler) addURLTestGroup(section string, sec *uci.Section, candidates []string, selectorTag, defaultTag string) error {
	ut, err := c.urlTestPlan(section, sec, candidates)
	if err != nil {
		return err
	}
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
		Tag:     URLTestTag(section),
		Kind:    OutboundURLTest,
		Members: candidates,
		URLTest: ut,
	})
	if selectorTag == "" {
		return nil
	}
	members := append(append([]string{}, candidates...), URLTestTag(section))
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
		Tag:     selectorTag,
		Kind:    OutboundSelector,
		Members: members,
		Default: defaultTag,
	})
	return nil
}

func (c *compiler) addManagedVPNFailover(section string, sec *uci.Section, primaryTag string, backupTags []string) error {
	if len(backupTags) == 0 {
		c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
			Tag:     OutboundTag(section),
			Kind:    OutboundSelector,
			Members: []string{primaryTag},
			Default: primaryTag,
		})
		return nil
	}
	ut, err := c.urlTestPlan(section, sec, backupTags)
	if err != nil {
		return err
	}
	urltestTag := URLTestTag(section)
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
		Tag:     urltestTag,
		Kind:    OutboundURLTest,
		Members: backupTags,
		URLTest: ut,
	})
	members := append([]string{primaryTag, urltestTag}, backupTags...)
	c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{
		Tag:     OutboundTag(section),
		Kind:    OutboundSelector,
		Members: members,
		Default: primaryTag,
	})
	return nil
}

func (c *compiler) urlTestPlan(section string, sec *uci.Section, candidates []string) (*URLTestPlan, error) {
	_ = section
	checkInterval := sec.Get("urltest_check_interval", "3m")
	idleTimeout := singbox.NormalizeDuration(sec.Get("urltest_idle_timeout", ""))
	if err := validation.ValidateURLTestDurationPair(checkInterval, idleTimeout); err != nil {
		return nil, err
	}
	tolerance := 50
	if v := sec.Get("urltest_tolerance", ""); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			tolerance = n
		}
	}
	_ = candidates
	return &URLTestPlan{
		URL:       sec.Get("urltest_testing_url", "https://www.gstatic.com/generate_204"),
		Interval:  checkInterval,
		Idle:      idleTimeout,
		Tolerance: tolerance,
		Interrupt: sec.GetBool("urltest_interrupt_exist_connections", false),
	}, nil
}

func (c *compiler) compileListRuleSets(section string, sec *uci.Section) error {
	conn := sec.Get("connection_type", "")
	outboundTag := OutboundTag(section)
	baseRule := RouteRule{
		Action:      "route",
		OutboundTag: outboundTag,
		Section:     section,
	}
	if conn == "block" {
		baseRule.Action = "reject"
		baseRule.Reject = true
		baseRule.OutboundTag = ""
	}
	// Ruleset tags per list key (community name, user, local, user:<name>),
	// so a binding can take some lists away from the section rule.
	byKey := make(map[string][]string)
	var keys []string
	add := func(key, tag string) {
		if _, ok := byKey[key]; !ok {
			keys = append(keys, key)
		}
		byKey[key] = append(byKey[key], tag)
	}
	for _, svc := range sec.GetList("community_lists") {
		svc = strings.TrimSpace(svc)
		if svc == "" {
			continue
		}
		domainsPath := filepath.Join(singbox.RulesetDir, RulesetTag(section, svc, "community")+".json")
		singbox.EnsureSourceRuleset(domainsPath)
		c.plan.RuleSets = append(c.plan.RuleSets, RuleSet{
			Tag:       RulesetTag(section, svc, "community"),
			Kind:      "domains",
			RemoteURL: singbox.CommunityServiceDomainURL(svc),
			Path:      domainsPath,
			FileStamp: rulesetFileStamp(domainsPath),
		})
		add(svc, RulesetTag(section, svc, "community"))
		if url, ok := singbox.SubnetListURLs[svc]; ok {
			lstPath := filepath.Join(singbox.RulesetDir, svc+".lst")
			_ = subnets.EnsureFile(url, lstPath)
			cidrs, err := subnets.ParseFile(lstPath)
			if err == nil && len(cidrs) > 0 {
				tag := RulesetTag(section, svc, "community-subnets")
				c.plan.RuleSets = append(c.plan.RuleSets, RuleSet{
					Tag:     tag,
					Kind:    "subnets",
					Subnets: cidrs,
					Path:    filepath.Join(singbox.RulesetDir, tag+".json"),
				})
				add(svc, tag)
			}
		}
	}
	if err := c.compileExtraDomainRuleSets(section, sec, add); err != nil {
		return err
	}
	if conn == "vpn" || conn == "proxy" {
		for _, ul := range listroutes.UserLists(c.pkg, section) {
			if len(ul.Domains) > 0 {
				tag, err := c.addDomainRuleSet(section, "ul-"+ul.Name, "domains", ul.Domains)
				if err != nil {
					return err
				}
				add(ul.Key(), tag)
			}
			if len(ul.Subnets) > 0 {
				add(ul.Key(), c.addSubnetRuleSet(section, "ul-"+ul.Name, ul.Subnets))
			}
		}
		taken := c.compileBindings(section, byKey)
		for _, k := range keys {
			if !taken[k] {
				baseRule.RuleSetTags = append(baseRule.RuleSetTags, byKey[k]...)
			}
		}
	} else {
		for _, k := range keys {
			baseRule.RuleSetTags = append(baseRule.RuleSetTags, byKey[k]...)
		}
	}
	if len(baseRule.RuleSetTags) > 0 {
		c.plan.Routes = append(c.plan.Routes, baseRule)
	}
	return nil
}

// compileBindings adds one rule per list_route of section, ahead of the
// section rule, and returns the list keys they took. Binding rules carry no
// Section, so the router dials their own outbound and not the section selector.
func (c *compiler) compileBindings(section string, byKey map[string][]string) map[string]bool {
	taken := make(map[string]bool)
	for _, r := range listroutes.Routes(c.pkg, section) {
		if !r.Bound() {
			continue
		}
		var lists, tags []string
		for _, l := range r.Lists {
			if taken[l] || len(byKey[l]) == 0 {
				continue
			}
			lists = append(lists, l)
			tags = append(tags, byKey[l]...)
		}
		if len(tags) == 0 {
			continue
		}
		bp := BindingPlan{Name: r.Name, Section: section, Lists: lists, Channel: r.Channel, OnDown: r.OnDown}
		rule := RouteRule{Action: "route", RuleSetTags: tags, Binding: r.Name}
		switch r.Channel {
		case listroutes.ChannelDirect:
			rule.OutboundTag = DirectTag
		case listroutes.ChannelBlock:
			rule.Action = "reject"
			rule.Reject = true
		case listroutes.ChannelBalance:
			members := c.channelTags(section)
			if len(members) == 0 {
				bp.Missing = true
				c.plan.Bindings = append(c.plan.Bindings, bp)
				continue
			}
			tag := BindingTag(section, r.Name)
			c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{Tag: tag, Kind: OutboundBalance, Members: members})
			rule.OutboundTag = tag
		default:
			chTag := c.channelTag(section, r.Channel)
			if chTag == "" {
				bp.Missing = true
				c.plan.Bindings = append(c.plan.Bindings, bp)
				continue
			}
			bp.ChannelTag = chTag
			members := []string{chTag}
			switch r.OnDown {
			case listroutes.DownPool:
				members = append(members, OutboundTag(section))
			case listroutes.DownDirect:
				members = append(members, DirectTag)
			}
			tag := BindingTag(section, r.Name)
			c.plan.Outbounds = append(c.plan.Outbounds, OutboundPlan{Tag: tag, Kind: OutboundFallback, Members: members})
			rule.OutboundTag = tag
		}
		bp.OutboundTag = rule.OutboundTag
		for _, l := range lists {
			taken[l] = true
		}
		c.plan.Routes = append(c.plan.Routes, rule)
		c.plan.Bindings = append(c.plan.Bindings, bp)
	}
	return taken
}

func (c *compiler) compileExtraDomainRuleSets(section string, sec *uci.Section, add func(key, tag string)) error {
	if domains := singbox.UserDomainItems(sec); len(domains) > 0 {
		tag, err := c.addDomainRuleSet(section, "user", "domains", domains)
		if err != nil {
			return err
		}
		add(listroutes.KeySectionUser, tag)
	}
	if cidrs := singbox.UserSubnetItems(sec); len(cidrs) > 0 {
		add(listroutes.KeySectionUser, c.addSubnetRuleSet(section, "user", cidrs))
	}
	for i, listPath := range sec.GetList("local_domain_lists") {
		listPath = strings.TrimSpace(listPath)
		if listPath == "" {
			continue
		}
		data, err := os.ReadFile(listPath)
		if err != nil {
			return fmt.Errorf("local domain list %q: %w", listPath, err)
		}
		domains := singbox.ParseDomainListBody(string(data))
		if len(domains) == 0 {
			continue
		}
		tag, err := c.addDomainRuleSet(section, fmt.Sprintf("local-%d", i), "domains", domains)
		if err != nil {
			return err
		}
		add(listroutes.KeySectionLocal, tag)
	}
	return nil
}

func (c *compiler) addDomainRuleSet(section, name, typ string, domains []string) (string, error) {
	tag := RulesetTag(section, name, typ)
	path := filepath.Join(singbox.RulesetDir, tag+".json")
	if err := singbox.WriteDomainRuleset(path, domains); err != nil {
		return "", err
	}
	c.plan.RuleSets = append(c.plan.RuleSets, RuleSet{
		Tag:       tag,
		Kind:      "domains",
		Domains:   domains,
		Path:      path,
		FileStamp: rulesetFileStamp(path),
	})
	return tag, nil
}

// addSubnetRuleSet keeps the CIDRs inline in the plan; the router reads them
// from there, no file needed.
func (c *compiler) addSubnetRuleSet(section, name string, cidrs []string) string {
	tag := RulesetTag(section, name, "subnets")
	c.plan.RuleSets = append(c.plan.RuleSets, RuleSet{
		Tag:     tag,
		Kind:    "subnets",
		Subnets: cidrs,
		Path:    filepath.Join(singbox.RulesetDir, tag+".json"),
	})
	return tag
}

func (c *compiler) compileRoutes() {
	for _, name := range c.pkg.SectionNames("section") {
		sec := c.pkg.Section(name)
		if sec == nil {
			continue
		}
		conn := sec.Get("connection_type", "")
		if conn != "vpn" && conn != "proxy" {
			continue
		}
		if sectionRoutesByLists(c.pkg, name, sec) {
			continue
		}
		c.plan.Routes = append(c.plan.Routes, RouteRule{
			Action:      "route",
			OutboundTag: OutboundTag(name),
			Section:     name,
		})
	}
}

// compileFullyRoutedRoutes sends all traffic from full_route clients through their section.
// Must run after list routes: router matches source CIDR before falling through to direct.
func (c *compiler) compileFullyRoutedRoutes() {
	bySection := clientrules.FullyRoutedBySection(clientrules.ListRules(c.pkg))
	if len(bySection) == 0 {
		return
	}
	extra := make([]RouteRule, 0, len(bySection))
	for section, ips := range bySection {
		if len(ips) == 0 {
			continue
		}
		extra = append(extra, RouteRule{
			Action:       "route",
			OutboundTag:  OutboundTag(section),
			Section:      section,
			SourceIPCIDR: append([]string(nil), ips...),
		})
	}
	if len(extra) == 0 {
		return
	}
	// Prefer source full-route before domain lists so these clients always use the section.
	c.plan.Routes = append(extra, c.plan.Routes...)
}

// compileUDPRoutedRoutes sends the UDP of udp_routed_ips clients through their
// section. nft marks that UDP into TPROXY (netlink.ApplyNFT), but without a
// source rule here the router fell through to the list logic and sent it
// direct, so the option only moved the traffic through the engine and back
// out the same WAN (console games cut by the ISP stayed cut).
func (c *compiler) compileUDPRoutedRoutes() {
	if c.pkg == nil {
		return
	}
	routable := make(map[string]bool, len(c.plan.Sections))
	for _, sec := range c.plan.Sections {
		if sec.ConnectionType == "vpn" || sec.ConnectionType == "proxy" {
			routable[sec.Name] = true
		}
	}
	var extra []RouteRule
	for _, name := range c.pkg.SectionNames("section") {
		sec := c.pkg.Section(name)
		if sec == nil || !routable[name] {
			continue
		}
		var ips []string
		for _, ip := range sec.GetList("udp_routed_ips") {
			if ip = strings.TrimSpace(ip); ip != "" {
				ips = append(ips, ip)
			}
		}
		if len(ips) == 0 {
			continue
		}
		extra = append(extra, RouteRule{
			Action:       "route",
			OutboundTag:  OutboundTag(name),
			Section:      name,
			SourceIPCIDR: ips,
			Network:      "udp",
		})
	}
	if len(extra) == 0 {
		return
	}
	c.plan.Routes = append(extra, c.plan.Routes...)
}

// ValidatePlan checks plan consistency.
func ValidatePlan(p *Plan) error {
	if p == nil {
		return fmt.Errorf("nil plan")
	}
	if len(p.Outbounds) == 0 {
		return fmt.Errorf("no outbounds")
	}
	seen := map[string]struct{}{}
	for _, ob := range p.Outbounds {
		if ob.Tag == "" {
			return fmt.Errorf("outbound missing tag")
		}
		if _, ok := seen[ob.Tag]; ok {
			return fmt.Errorf("duplicate outbound tag %q", ob.Tag)
		}
		seen[ob.Tag] = struct{}{}
	}
	return nil
}

func planHashPath() string {
	return filepath.Join(filepath.Dir(singbox.RulesetDir), "engine-plan.sha256")
}

func writePlanMarker() error {
	return os.MkdirAll(filepath.Dir(planHashPath()), 0o755)
}

// rulesetFileStamp captures on-disk identity so Hash() changes when stubs are filled.
func rulesetFileStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", st.ModTime().UnixNano(), st.Size())
}
