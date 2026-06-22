package singbox

import "strings"

// Geo and category lists live outside Services/*.lst in allow-domains.
var communityDomainListPaths = map[string]string{
	"russia_inside":  "Russia/inside-clashx.lst",
	"russia_outside": "Russia/outside-clashx.lst",
	"ukraine_inside": "Ukraine/inside-clashx.lst",
	"geoblock":       "Categories/geoblock.lst",
	"block":          "Categories/block.lst",
	"porn":           "Categories/porn.lst",
	"news":           "Categories/news.lst",
	"anime":          "Categories/anime.lst",
	"hodca":          "Categories/hodca.lst",
}

func CommunityServiceDomainURL(service string) string {
	if rel, ok := communityDomainListPaths[service]; ok {
		return GitHubRawURL + "/" + rel
	}
	return GitHubRawURL + "/Services/" + service + ".lst"
}

// ParseDomainListBody extracts domain suffixes from plain or Clash-style list files.
func ParseDomainListBody(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	seen := make(map[string]struct{})
	var out []string
	add := func(domain string) {
		domain = strings.TrimSpace(domain)
		domain = strings.TrimPrefix(domain, ".")
		domain = strings.ToLower(domain)
		if domain == "" {
			return
		}
		if _, ok := seen[domain]; ok {
			return
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ",") {
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			switch strings.TrimSpace(parts[0]) {
			case "DOMAIN-SUFFIX", "DOMAIN-KEYWORD", "DOMAIN":
				add(parts[1])
			default:
				continue
			}
			continue
		}
		add(line)
	}
	return out
}
