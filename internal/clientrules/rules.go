package clientrules

import (
	"fmt"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

const (
	ModeInclude       = "include"
	ModeExclude       = "exclude"
	ModeFullRoute     = "full_route"
	ModeGlobalExclude = "global_exclude"
)

// Rule is one unified client routing rule.
type Rule struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	Mode    string `json:"mode"`
	Section string `json:"section,omitempty"`
}

// ListRules returns unified rules from client_rule sections and legacy lists.
func ListRules(pkg *uci.Package) []Rule {
	if pkg == nil {
		return nil
	}
	var rules []Rule
	for _, name := range pkg.SectionNames("client_rule") {
		sec := pkg.Section(name)
		if sec == nil {
			continue
		}
		ip := strings.TrimSpace(sec.Get("ip", ""))
		if ip == "" {
			continue
		}
		mode := normalizeMode(sec.Get("mode", ""))
		if mode == "" {
			continue
		}
		rules = append(rules, Rule{
			Name:    name,
			IP:      ip,
			Mode:    mode,
			Section: sec.Get("section", ""),
		})
	}
	if len(rules) > 0 {
		return rules
	}
	return legacyRules(pkg)
}

// LegacyRulesFromPackage reads legacy list options when no client_rule sections exist.
func LegacyRulesFromPackage(pkg *uci.Package) []Rule {
	return legacyRules(pkg)
}

func legacyRules(pkg *uci.Package) []Rule {
	var rules []Rule
	if settings := pkg.Section("settings"); settings != nil {
		for i, ip := range settings.GetList("include_source_ips") {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			rules = append(rules, Rule{Name: fmt.Sprintf("legacy-include-%d", i), IP: ip, Mode: ModeInclude})
		}
		for i, ip := range settings.GetList("exclude_source_ips") {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			rules = append(rules, Rule{Name: fmt.Sprintf("legacy-exclude-%d", i), IP: ip, Mode: ModeExclude})
		}
		for i, ip := range settings.GetList("routing_excluded_ips") {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			rules = append(rules, Rule{Name: fmt.Sprintf("legacy-global-exclude-%d", i), IP: ip, Mode: ModeGlobalExclude})
		}
	}
	for _, secName := range pkg.SectionNames("section") {
		sec := pkg.Section(secName)
		if sec == nil {
			continue
		}
		for i, ip := range sec.GetList("fully_routed_ips") {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			rules = append(rules, Rule{
				Name:    fmt.Sprintf("legacy-full-%s-%d", secName, i),
				IP:      ip,
				Mode:    ModeFullRoute,
				Section: secName,
			})
		}
	}
	return rules
}

func normalizeMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "include", "in":
		return ModeInclude
	case "exclude", "out":
		return ModeExclude
	case "full_route", "full", "fully_routed":
		return ModeFullRoute
	case "global_exclude", "routing_excluded":
		return ModeGlobalExclude
	default:
		return ""
	}
}

// IncludeIPs returns IPs that should receive tproxy mark.
func IncludeIPs(rules []Rule) []string {
	var out []string
	for _, r := range rules {
		switch r.Mode {
		case ModeInclude, ModeFullRoute:
			out = append(out, r.IP)
		}
	}
	return out
}

// ExcludeIPs returns IPs that bypass hybrid failover mark.
func ExcludeIPs(rules []Rule) []string {
	var out []string
	for _, r := range rules {
		if r.Mode == ModeExclude {
			out = append(out, r.IP)
		}
	}
	return out
}

// GlobalExcludeIPs returns destination exclusions for sing-box route.
func GlobalExcludeIPs(rules []Rule) []string {
	var out []string
	for _, r := range rules {
		if r.Mode == ModeGlobalExclude {
			out = append(out, r.IP)
		}
	}
	return out
}

// FullyRoutedBySection returns fully routed IPs grouped by section.
func FullyRoutedBySection(rules []Rule) map[string][]string {
	out := make(map[string][]string)
	for _, r := range rules {
		if r.Mode != ModeFullRoute || r.Section == "" {
			continue
		}
		out[r.Section] = append(out[r.Section], r.IP)
	}
	return out
}
