package subnets_test

import (
	"slices"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/subnets"
)

func TestNormalizeForNFTDropsSubsets(t *testing.T) {
	in := []string{
		"128.116.0.0/17",
		"128.116.0.0/24",
		"149.154.160.0/20",
		"198.18.0.0/16",
	}
	got := subnets.NormalizeForNFT(in)
	if slices.Contains(got, "128.116.0.0/24") {
		t.Fatalf("subset should be removed: %v", got)
	}
	if !slices.Contains(got, "128.116.0.0/17") || !slices.Contains(got, "149.154.160.0/20") {
		t.Fatalf("expected parent and telegram cidr: %v", got)
	}
	if slices.Contains(got, "198.18.0.0/16") {
		t.Fatalf("fakeip range should be excluded: %v", got)
	}
}
