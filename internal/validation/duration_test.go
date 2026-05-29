package validation

import "testing"

func TestNormalizeSingboxDurationAddsSeconds(t *testing.T) {
	got, err := NormalizeSingboxDuration("60")
	if err != nil {
		t.Fatal(err)
	}
	if got != "60s" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeSingboxDurationKeepsUnit(t *testing.T) {
	got, err := NormalizeSingboxDuration("5m")
	if err != nil || got != "5m" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestNormalizeUCIOptionValueForIdleTimeout(t *testing.T) {
	got, err := NormalizeUCIOptionValue("hybrid-failover.glob.urltest_idle_timeout", "45")
	if err != nil || got != "45s" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestNormalizeUCIOptionValueIgnoresTolerance(t *testing.T) {
	got, err := NormalizeUCIOptionValue("hybrid-failover.glob.urltest_tolerance", "50")
	if err != nil || got != "50" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestIsSingboxDurationUCIKey(t *testing.T) {
	if !IsSingboxDurationUCIKey("hybrid-failover.glob.urltest_check_interval") {
		t.Fatal("expected duration key")
	}
	if IsSingboxDurationUCIKey("hybrid-failover.glob.urltest_tolerance") {
		t.Fatal("tolerance is not a duration")
	}
}
