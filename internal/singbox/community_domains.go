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
		"youtube.com",
		"youtu.be",
		"yt.be",
		"youtube-nocookie.com",
		"youtubekids.com",
		"ytimg.com",
		"ggpht.com",
		"googlevideo.com",
		// Specific Google hosts for YouTube login / clients (not bare google.com).
		"accounts.google.com",
		"play.google.com",
		"www.google.com",
		"clients1.google.com",
		"clients3.google.com",
		"clients4.google.com",
		"clients6.google.com",
		"googleapis.com",
		"gstatic.com",
		"googleusercontent.com",
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
	// russia_inside includes youtube.com but not Google hosts used by YouTube login.
	"russia_inside": {
		"accounts.google.com",
		"play.google.com",
		"www.google.com",
		"clients1.google.com",
		"clients3.google.com",
		"clients4.google.com",
		"clients6.google.com",
		"googleapis.com",
		"gstatic.com",
		"googleusercontent.com",
		"gvt1.com",
	},
	// European Xbox in RF: only Microsoft auth through the tunnel (0x80a40401).
	// Do NOT FakeIP Epic/Psyonix/RL: matchmaking via foreign exit assigns Asia
	// dedicated servers while game UDP stays direct → join hang / connection lost.
	// Do NOT suffix-match xboxlive.com / xbox.com (party/Teredo → RL error 71).
	"rocketleague": {
		"xsts.auth.xboxlive.com",
		"user.auth.xboxlive.com",
		"title.auth.xboxlive.com",
		"login.live.com",
		"account.live.com",
	},
}

// CommunityDomainRemovals drops domains that must not stay in on-disk rulesets
// after they were removed from supplements (EnsureCommunitySupplements merges
// existing files and would otherwise keep them forever).
var CommunityDomainRemovals = map[string][]string{
	"rocketleague": {
		"rocketleague.com",
		"psyonix.com",
		"rl-psy.net",
		"psy.net",
		"psynet.gg",
		"datahound.com",
		"epicgames.com",
		"epicgames.dev",
		"unrealengine.com",
		"gamepass.com",
		"xbox.com",
		"xboxlive.com",
		"xboxservices.com",
		"xboxab.com",
		"xboxab.net",
		"xboxservice.com",
		"packages.xboxlive.com",
		"portalservices.xboxlive.com",
		"titlestorage.xboxlive.com",
		"accounts.xboxlive.com",
		"xbl-smooth.xboxlive.com",
	},
}

func MergeCommunityDomains(service string, domains []string) []string {
	extra, ok := CommunityDomainSupplements[service]
	if !ok || len(extra) == 0 {
		return filterCommunityRemovals(service, domains)
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
	return filterCommunityRemovals(service, out)
}

func filterCommunityRemovals(service string, domains []string) []string {
	drop := CommunityDomainRemovals[service]
	if len(drop) == 0 || len(domains) == 0 {
		return domains
	}
	deny := make(map[string]struct{}, len(drop))
	for _, d := range drop {
		deny[strings.ToLower(d)] = struct{}{}
	}
	out := make([]string, 0, len(domains))
	for _, d := range domains {
		if _, bad := deny[strings.ToLower(d)]; bad {
			continue
		}
		out = append(out, d)
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
