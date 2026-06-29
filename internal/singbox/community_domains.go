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

// CommunityDomainSupplements adds domains required by apps but missing from upstream .lst files.
var CommunityDomainSupplements = map[string][]string{
	"youtube": {
		"googleapis.com",
		"gstatic.com",
		"googleusercontent.com",
		"play.google.com",
		"clients1.google.com",
		"clients3.google.com",
		"clients6.google.com",
		"gvt1.com",
		"gemini.google.com",
		"ai.google.dev",
		"google.ai",
		"aistudio.google.com",
		"bard.google.com",
		"makersuite.google.com",
		"notebooklm.google.com",
		"aiplatform.googleapis.com",
		"alkalimakersuite-pa.clients6.google.com",
		"proactivebackend-pa.googleapis.com",
	},
}

func MergeCommunityDomains(service string, domains []string) []string {
	extra, ok := CommunityDomainSupplements[service]
	if !ok || len(extra) == 0 {
		return domains
	}
	seen := make(map[string]struct{}, len(domains)+len(extra))
	out := make([]string, 0, len(domains)+len(extra))
	add := func(d string) {
		d = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(d), "."))
		if d == "" {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	for _, d := range domains {
		add(d)
	}
	for _, d := range extra {
		add(d)
	}
	return out
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
