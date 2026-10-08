package routing

import "testing"

func TestMaskURLKeepsHostHidesCredentials(t *testing.T) {
	cases := map[string]string{
		"https://sub.example.com/api/v1/s/SECRETTOKEN1234":        "https://sub.example.com/…1234",
		"https://sub.example.com":                                 "https://sub.example.com",
		"https://sub.example.com/x":                               "https://sub.example.com/…",
		"vless://11111111-2222@host.net:443?type=ws&sni=a#name":   "vless://•••@host.net:443/…name",
		"https://user:pass@sub.example.com/path/token?x=abcdefgh": "https://•••@sub.example.com/…efgh",
		"not a link": "not a link",
	}
	for in, want := range cases {
		if got := MaskURL(in); got != want {
			t.Errorf("MaskURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskSecretsFindsLinksInText(t *testing.T) {
	in := "a='https://sub.example.com/s/SECRETTOKEN1234' b=vless://uuid@h:1?k=v end"
	got := MaskSecrets(in)
	for _, leak := range []string{"SECRETTOKEN", "uuid", "k=v"} {
		if contains(got, leak) {
			t.Fatalf("%q leaked in %q", leak, got)
		}
	}
}

func TestValidateSubscriptionURL(t *testing.T) {
	good := []string{"https://a.example/sub", "http://1.2.3.4:8080/x?y=1"}
	bad := []string{"", "ftp://a/x", "https://", "https://a/x y", "https://a/x'; rm -rf /", "javascript:alert(1)", "vless://u@h:1"}
	for _, g := range good {
		if err := ValidateSubscriptionURL(g); err != nil {
			t.Errorf("%q rejected: %v", g, err)
		}
	}
	for _, b := range bad {
		if err := ValidateSubscriptionURL(b); err == nil {
			t.Errorf("%q accepted", b)
		}
	}
}

func TestValidateSubscriptionInterval(t *testing.T) {
	for _, v := range []string{"30m", "6h", "12h", "1d"} {
		if err := ValidateSubscriptionInterval(v); err != nil {
			t.Errorf("%q rejected", v)
		}
	}
	for _, v := range []string{"", "0h", "h", "6", "6x", "6h; reboot", "-1h"} {
		if err := ValidateSubscriptionInterval(v); err == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
