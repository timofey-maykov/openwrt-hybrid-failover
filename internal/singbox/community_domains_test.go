package singbox

import "testing"

func TestCommunityServiceDomainURL(t *testing.T) {
	if got := CommunityServiceDomainURL("netflix"); got != "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/netflix.list" {
		t.Fatalf("Netflix source: %s", got)
	}
	if got := CommunityServiceDomainURL("russia_inside"); got != GitHubRawURL+"/Russia/inside-clashx.lst" {
		t.Fatalf("russia_inside URL = %q", got)
	}
	if got := CommunityServiceDomainURL("meta"); got != GitHubRawURL+"/Services/meta.lst" {
		t.Fatalf("meta URL = %q", got)
	}
}

func TestMergeCommunityDomains(t *testing.T) {
	got := MergeCommunityDomains("youtube", []string{"youtube.com"})
	if len(got) < 18 {
		t.Fatalf("expected supplements merged, got %v", got)
	}
	seen := make(map[string]bool)
	for _, d := range got {
		seen[d] = true
	}
	for _, want := range []string{
		"youtube.com", "accounts.google.com", "googleapis.com", "gstatic.com",
		"gemini.google.com", "ai.google.dev",
	} {
		if !seen[want] {
			t.Fatalf("missing %q in %v", want, got)
		}
	}
	if seen["google.com"] {
		t.Fatalf("bare google.com must not be a youtube supplement: %v", got)
	}
	gotRI := MergeCommunityDomains("russia_inside", []string{"youtube.com"})
	seenRI := make(map[string]bool)
	for _, d := range gotRI {
		seenRI[d] = true
	}
	if !seenRI["accounts.google.com"] {
		t.Fatalf("russia_inside missing accounts.google.com in %v", gotRI)
	}
	if seenRI["google.com"] {
		t.Fatalf("bare google.com must not be a russia_inside supplement: %v", gotRI)
	}
}

func TestMergeCommunityDomainsRocketLeagueNoXbox(t *testing.T) {
	got := MergeCommunityDomains("rocketleague", []string{
		"rocketleague.com", "xboxlive.com", "xbox.com", "gamepass.com", "epicgames.com",
	})
	seen := make(map[string]bool)
	for _, d := range got {
		seen[d] = true
	}
	if !seen["xsts.auth.xboxlive.com"] || !seen["login.live.com"] {
		t.Fatalf("expected Xbox auth hosts for RF, got %v", got)
	}
	for _, bad := range []string{
		"xboxlive.com", "xbox.com", "gamepass.com", "xboxservices.com",
		"rocketleague.com", "epicgames.com", "psyonix.com",
	} {
		if seen[bad] {
			t.Fatalf("domain %q must not be FakeIP for RL path, got %v", bad, got)
		}
	}
}

func TestParseDomainListBody(t *testing.T) {
	raw := "instagram.com\nDOMAIN-SUFFIX,cdninstagram.com\nDOMAIN-SUFFIX,.ua\n# comment\n"
	got := ParseDomainListBody(raw)
	want := map[string]bool{"instagram.com": true, "cdninstagram.com": true, "ua": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, d := range got {
		if !want[d] {
			t.Fatalf("unexpected domain %q in %v", d, got)
		}
	}
}

func TestNetflixMihomoSuffixesAndFallback(t *testing.T) {
	if got := CommunitySRSURL("netflix"); got != "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/netflix.srs" {
		t.Fatalf("Netflix SRS: %s", got)
	}
	got := ParseDomainListBody("+.netflix.com\n+.nflxvideo.net\n# comment\n")
	if len(got) != 2 || got[0] != "netflix.com" || got[1] != "nflxvideo.net" {
		t.Fatalf("invalid suffixes: %v", got)
	}
	if got := MergeCommunityDomains("netflix", nil); len(got) < 7 {
		t.Fatal("Netflix fallback is empty")
	}
}
