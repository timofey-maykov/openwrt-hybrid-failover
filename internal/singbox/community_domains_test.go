package singbox

import "testing"

func TestCommunityServiceDomainURL(t *testing.T) {
	if got := CommunityServiceDomainURL("russia_inside"); got != GitHubRawURL+"/Russia/inside-clashx.lst" {
		t.Fatalf("russia_inside URL = %q", got)
	}
	if got := CommunityServiceDomainURL("meta"); got != GitHubRawURL+"/Services/meta.lst" {
		t.Fatalf("meta URL = %q", got)
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
