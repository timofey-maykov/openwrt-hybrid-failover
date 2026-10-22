package secrets

import (
	"strings"
	"testing"
)

func TestMaskURL(t *testing.T) {
	cases := map[string]string{
		"https://sub.example.com/api/v1/s/SECRETTOKEN1234":       "https://sub.example.com/…1234",
		"https://sub.example.com":                                "https://sub.example.com",
		"vless://11111111-2222@host.net:443?type=ws&sni=a#name":  "vless://•••@host.net:443/…name",
		"awg2://1.2.3.4:41490?private_key=SECRET&public_key=PUB": "awg2://1.2.3.4:41490/…=PUB",
		"not a link": "not a link",
		// Already masked text goes through status files and notices twice.
		"https://sub.example.com/…1234:": "https://sub.example.com/…1234:",
		"vless://•••@host.net:443/…name": "vless://•••@host.net:443/…name",
	}
	for in, want := range cases {
		if got := MaskURL(in); got != want {
			t.Errorf("MaskURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskSecretsCoversEverySchemeTheRouterUses(t *testing.T) {
	in := "a='https://p/s/TOKEN1' b=awg2://h:1?private_key=PK c=amneziawg://BASE64BODY#n d=vless://uuid@h:1?k=v"
	got := MaskSecrets(in)
	for _, leak := range []string{"TOKEN1", "private_key=PK", "BASE64BODY", "uuid", "k=v"} {
		if strings.Contains(got, leak) {
			t.Fatalf("%q leaked in %q", leak, got)
		}
	}
}

func TestMaskBotToken(t *testing.T) {
	// Built at run time so the source holds nothing that looks like a token.
	token := "1234567890" + ":" + strings.Repeat("x", 35)
	in := `Post "https://api.telegram.org/bot` + token + `/getMe": dial tcp: i/o timeout`
	want := `Post "https://api.telegram.org/bot<token>/getMe": dial tcp: i/o timeout`
	if got := MaskBotToken(in); got != want {
		t.Fatalf("got %q", got)
	}
	if got := MaskBotToken("router 192.168.1.1:53 timeout"); got != "router 192.168.1.1:53 timeout" {
		t.Fatalf("plain text changed: %q", got)
	}
}
