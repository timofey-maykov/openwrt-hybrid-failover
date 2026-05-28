package validation

import "testing"

func TestValidateUCIKeyAcceptsPodkopOption(t *testing.T) {
	cases := []string{
		"podkop.glob.failover_proxy_links",
		"podkop.settings.disable_quic",
		"podkop.@section[0].enabled",
	}
	for _, key := range cases {
		if err := ValidateUCIKey(key); err != nil {
			t.Fatalf("expected %q valid, got %v", key, err)
		}
	}
}

func TestValidateUCIKeyRejectsUnsafeTargets(t *testing.T) {
	cases := []string{
		"",
		"network.lan.ipaddr",
		"podkop.glob.failover_proxy_links; reboot",
		"podkop.glob.failover_proxy_links && rm -rf /",
	}
	for _, key := range cases {
		if err := ValidateUCIKey(key); err == nil {
			t.Fatalf("expected %q invalid", key)
		}
	}
}

func TestNormalizeValue(t *testing.T) {
	if got := NormalizeValue("  value  "); got != "value" {
		t.Fatalf("unexpected normalize result: %q", got)
	}
}
