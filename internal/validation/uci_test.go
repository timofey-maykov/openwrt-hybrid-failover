package validation

import "testing"

func TestValidateUCIKey(t *testing.T) {
	valid := []string{
		"hybrid-failover.glob.failover_proxy_links",
		"hybrid-failover.settings.disable_quic",
		"hybrid-failover.@section[0].enabled",
	}
	for _, k := range valid {
		if err := ValidateUCIKey(k); err != nil {
			t.Fatalf("expected valid %q: %v", k, err)
		}
	}
	invalid := []string{
		"hybrid-failover.glob.failover_proxy_links; reboot",
		"hybrid-failover.glob.failover_proxy_links && rm -rf /",
	}
	for _, k := range invalid {
		if err := ValidateUCIKey(k); err == nil {
			t.Fatalf("expected invalid %q", k)
		}
	}
}
