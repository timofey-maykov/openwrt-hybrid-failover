package singbox

import "testing"

func TestDropCloudflareSupernets(t *testing.T) {
	in := []string{
		"104.16.0.0/12",
		"172.64.0.0/13",
		"162.158.0.0/15",
		"20.192.0.0/10",
		"91.108.4.0/22",
		"104.18.124.0/24",
	}
	got := dropCloudflareSupernets(in)
	seen := map[string]bool{}
	for _, c := range got {
		seen[c] = true
	}
	for _, bad := range []string{"104.16.0.0/12", "172.64.0.0/13", "162.158.0.0/15", "104.18.124.0/24"} {
		if seen[bad] {
			t.Fatalf("cloudflare range %s must be dropped, got %v", bad, got)
		}
	}
	if !seen["20.192.0.0/10"] || !seen["91.108.4.0/22"] {
		t.Fatalf("non-CF ranges must remain, got %v", got)
	}
}
