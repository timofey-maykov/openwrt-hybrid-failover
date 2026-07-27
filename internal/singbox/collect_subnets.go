package singbox

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

// cloudflareSupernetsForNFT are shared Cloudflare ranges pulled in by discord.lst
// (and similar). api.epicgames.dev sits on the same space; marking it for tproxy
// sends console matchmaking through the tunnel while game UDP stays direct, so
// Rocket League lands on Asia dedicated servers and drops. Discord/meta/twitter
// already use FakeIP domain routing and do not need these IP CIDRs in nft.
var cloudflareSupernetsForNFT = []string{
	"104.16.0.0/12",
	"172.64.0.0/13",
	"162.158.0.0/15",
}

// CollectProxySubnets returns IPv4 CIDRs that should enter tproxy from LAN clients.
func CollectProxySubnets(pkg *uci.Package) []string {
	if pkg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(cidrs ...string) {
		for _, cidr := range cidrs {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				continue
			}
			if _, ok := seen[cidr]; ok {
				continue
			}
			seen[cidr] = struct{}{}
			out = append(out, cidr)
		}
	}

	for _, name := range pkg.SectionNames("section") {
		sec := pkg.Section(name)
		if sec == nil || !SectionHasEnabledLists(sec) {
			continue
		}
		for _, svc := range sec.GetList("community_lists") {
			svc = strings.TrimSpace(svc)
			if url, ok := SubnetListURLs[svc]; ok {
				path := filepath.Join(RulesetDir, svc+".lst")
				_ = subnets.EnsureFile(url, path)
				if cidrs, err := subnets.ParseFile(path); err == nil {
					add(cidrs...)
				}
			}
		}
		add(subnets.SplitItems(sec.Get("user_subnets_text", ""))...)
		add(sec.GetList("user_subnets")...)
		for _, path := range sec.GetList("local_subnet_lists") {
			add(readSubnetPath(path)...)
		}
		for _, rawURL := range sec.GetList("remote_subnet_lists") {
			add(readRemoteSubnetURL(name, rawURL)...)
		}
	}
	return dropCloudflareSupernets(out)
}

func dropCloudflareSupernets(cidrs []string) []string {
	var deny []*net.IPNet
	for _, raw := range cloudflareSupernetsForNFT {
		_, n, err := net.ParseCIDR(raw)
		if err == nil {
			deny = append(deny, n)
		}
	}
	out := make([]string, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		skip := false
		for _, d := range deny {
			// Exact supernet from discord.lst, or any more-specific CF piece.
			if d.String() == n.String() || d.Contains(n.IP) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, cidr)
		}
	}
	return out
}

func readSubnetPath(listPath string) []string {
	path := listPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(RulesetDir, filepath.Base(path))
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".lst":
		cidrs, err := subnets.ParseFile(path)
		if err != nil {
			return nil
		}
		return cidrs
	case ".json":
		return parseSubnetRulesetJSON(path)
	default:
		cidrs, err := subnets.ParseFile(path)
		if err == nil && len(cidrs) > 0 {
			return cidrs
		}
		return parseSubnetRulesetJSON(path)
	}
}

func readRemoteSubnetURL(section, rawURL string) []string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(rawURL, "?")[0]))
	switch ext {
	case ".lst":
		path := filepath.Join(RulesetDir, RulesetTag(section, "remote", "subnets")+".lst")
		_ = subnets.EnsureFile(rawURL, path)
		if cidrs, err := subnets.ParseFile(path); err == nil && len(cidrs) > 0 {
			return cidrs
		}
		path = filepath.Join(RulesetDir, RulesetTag(section, "remote", "subnets")+".json")
		return parseSubnetRulesetJSON(path)
	default:
		path := filepath.Join(RulesetDir, RulesetTag(section, "remote", "subnets")+".json")
		return parseSubnetRulesetJSON(path)
	}
}

func parseSubnetRulesetJSON(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Rules []struct {
			IPCIDR []string `json:"ip_cidr"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var out []string
	for _, rule := range doc.Rules {
		out = append(out, rule.IPCIDR...)
	}
	return out
}
