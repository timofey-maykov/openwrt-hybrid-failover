package validation

import "testing"

func TestValidateProxyURI_AcceptsSupportedSchemes(t *testing.T) {
	cases := []string{
		"vless://uuid@example.com:443?security=tls#name",
		"trojan://password@example.com:443#name",
		"ss://YWVzLTI1Ni1nY206cGFzc0BleGFtcGxlLmNvbTo0NDM=#name",
		"vpn://AAAABBBB",
	}
	for _, uri := range cases {
		if err := ValidateProxyURI(uri); err != nil {
			t.Fatalf("expected %q to be valid, got %v", uri, err)
		}
	}
}

func TestValidateProxyURI_RejectsUnsupportedScheme(t *testing.T) {
	if err := ValidateProxyURI("http://example.com"); err == nil {
		t.Fatal("expected unsupported scheme to fail")
	}
}

func TestValidateProxyURI_RejectsEmpty(t *testing.T) {
	if err := ValidateProxyURI(""); err == nil {
		t.Fatal("expected empty uri to fail")
	}
}
