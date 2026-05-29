package singbox

import "strconv"

func parseDNSRewriteTTL(raw string) int {
	if raw == "" {
		raw = DefaultDNSRewriteTTL
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 60
	}
	return n
}

func (b *Builder) configureDNS() {
	settings := b.pkg.Section("settings")
	dnsType := DefaultDNSType
	dnsServer := DefaultDNSServer
	bootstrap := DefaultBootstrapDNS
	rewriteTTL := parseDNSRewriteTTL("")
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
			rewriteTTL = parseDNSRewriteTTL(v)
		}
	}

	var domainResolver string
	if !isIPv4(urlHost(dnsServer)) {
		domainResolver = BootstrapTag
	}

	servers := []map[string]any{
		b.bootstrapUDPServer(bootstrap),
	}
	servers = append(servers, b.dnsServerEntry(dnsType, dnsServer, domainResolver)...)
	for _, resolver := range b.sectionDomainResolvers {
		servers = append(servers, b.dnsServerEntryWithDetour(
			resolver.dnsType,
			resolver.dnsServer,
			resolver.domainResolver,
			resolver.detour,
			resolver.tag,
		)...)
	}
	servers = append(servers, map[string]any{
		"tag":         FakeIPTag,
		"type":        "fakeip",
		"inet4_range": FakeIPInet4Range,
	})

	rules := []map[string]any{
		{"action": "reject", "query_type": []string{"HTTPS"}},
		{"action": "reject", "domain_suffix": []string{"use-application-dns.net"}},
		{
			"action":       "route",
			"server":       FakeIPTag,
			"rewrite_ttl":  rewriteTTL,
			"domain":       []string{FAKEIPTestDomain, CheckProxyIPDomain},
			"__dns_rule__": FakeIPDNSRuleTag,
		},
	}

	b.cfg.DNS = map[string]any{
		"servers":           servers,
		"rules":             rules,
		"final":             DNSServerTag,
		"strategy":          "ipv4_only",
		"independent_cache": true,
	}
}

func (b *Builder) bootstrapUDPServer(bootstrap string) map[string]any {
	return map[string]any{
		"tag":         BootstrapTag,
		"type":        "udp",
		"server":      bootstrap,
		"server_port": 53,
	}
}

func (b *Builder) dnsServerEntry(dnsType, dnsServer, domainResolver string) []map[string]any {
	return b.dnsServerEntryWithDetour(dnsType, dnsServer, domainResolver, "", DNSServerTag)
}

func (b *Builder) dnsServerEntryWithDetour(dnsType, dnsServer, domainResolver, detour, tag string) []map[string]any {
	host := urlHost(dnsServer)
	port := urlPort(dnsServer, 0)
	entry := map[string]any{
		"tag": tag,
	}
	if domainResolver != "" {
		entry["domain_resolver"] = domainResolver
	}
	if detour != "" {
		entry["detour"] = detour
	}

	switch dnsType {
	case "udp":
		if port == 0 {
			port = 53
		}
		entry["type"] = "udp"
		entry["server"] = host
		entry["server_port"] = port
	case "dot":
		if port == 0 {
			port = 853
		}
		entry["type"] = "tls"
		entry["server"] = host
		entry["server_port"] = port
	default:
		if port == 0 {
			port = 443
		}
		entry["type"] = "https"
		entry["server"] = host
		entry["server_port"] = port
		p := urlPath(dnsServer)
		if p == "" {
			p = "/dns-query"
		}
		entry["path"] = p
	}
	return []map[string]any{entry}
}

func (b *Builder) patchFakeIPDNSRule(key string, value any) {
	rules, _ := b.cfg.DNS["rules"].([]map[string]any)
	for i, rule := range rules {
		if rule["__dns_rule__"] == FakeIPDNSRuleTag {
			if existing, ok := rule[key]; ok {
				if key == "rule_set" {
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
			} else {
				rule[key] = value
			}
			rules[i] = rule
			b.cfg.DNS["rules"] = rules
			return
		}
	}
}
