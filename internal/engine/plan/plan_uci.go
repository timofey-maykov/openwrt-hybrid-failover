package plan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/amnezia"
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
	return c.plan, nil
}

type compiler struct {
	pkg  *uci.Package
	plan *Plan
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
	c.plan.Sections = append(c.plan.Sections, sp)
	if singbox.SectionHasEnabledLists(sec) {
		if err := c.compileListRuleSets(section, sec); err != nil {
			return err
		}
	}
	return nil
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
			candidates = append(candidates, tag)
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
		tag := OutboundTag(section)
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
	if conn == "block" {
		c.plan.Routes = append(c.plan.Routes, RouteRule{Action: "reject", Reject: true, Section: section})
		return nil
	}
	baseRule := RouteRule{
		Action:      "route",
		OutboundTag: outboundTag,
		Section:     section,
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
		baseRule.RuleSetTags = append(baseRule.RuleSetTags, RulesetTag(section, svc, "community"))
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
				baseRule.RuleSetTags = append(baseRule.RuleSetTags, tag)
			}
		}
	}
	if err := c.compileExtraDomainRuleSets(section, sec, &baseRule); err != nil {
		return err
	}
	if len(baseRule.RuleSetTags) > 0 {
		c.plan.Routes = append(c.plan.Routes, baseRule)
	}
	return nil
}

func (c *compiler) compileExtraDomainRuleSets(section string, sec *uci.Section, baseRule *RouteRule) error {
	if domains := singbox.UserDomainItems(sec); len(domains) > 0 {
		if err := c.addDomainRuleSet(section, "user", "domains", domains, baseRule); err != nil {
			return err
		}
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
		name := fmt.Sprintf("local-%d", i)
		if err := c.addDomainRuleSet(section, name, "domains", domains, baseRule); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) addDomainRuleSet(section, name, typ string, domains []string, baseRule *RouteRule) error {
	tag := RulesetTag(section, name, typ)
	path := filepath.Join(singbox.RulesetDir, tag+".json")
	if err := singbox.WriteDomainRuleset(path, domains); err != nil {
		return err
	}
	c.plan.RuleSets = append(c.plan.RuleSets, RuleSet{
		Tag:       tag,
		Kind:      "domains",
		Domains:   domains,
		Path:      path,
		FileStamp: rulesetFileStamp(path),
	})
	baseRule.RuleSetTags = append(baseRule.RuleSetTags, tag)
	return nil
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
		if singbox.SectionHasEnabledLists(sec) {
			continue
		}
		c.plan.Routes = append(c.plan.Routes, RouteRule{
			Action:      "route",
			OutboundTag: OutboundTag(name),
			Section:     name,
		})
	}
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
