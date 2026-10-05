package amnezia

import (
	"encoding/base64"
	"net/url"
	"testing"
)

const testConf = `[Interface]
PrivateKey = cHJpdmF0ZS1rZXktZm9yLXRlc3RzLW9ubHktMDAwMDA=
Address = 10.78.0.2/32
MTU = 1280
DNS = 1.1.1.1, 8.8.8.8
Jc = 4
Jmin = 10
Jmax = 50
S1 = 67
S2 = 66
S3 = 48
S4 = 12
H1 = 1
H2 = 2
H3 = 3
H4 = 4
HeaderProtectionKey = aGVhZGVyLXByb3RlY3Rpb24ta2V5LXRlc3Qtb25seS0w
ContentPaddingAddition = 10-100
RandomTrailers = on
DisableCookies = on
RekeyAfterTime = 100-120
MaxHandshakeAttempts = 15-20

[Peer]
PublicKey = c2VydmVyLXB1YmxpYy1rZXktZm9yLXRlc3RzLW9ubHkwMDA=
PresharedKey = cHJlc2hhcmVkLWtleS1mb3ItdGVzdHMtb25seS0wMDAwMDA=
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 192.0.2.10:41491
PersistentKeepalive = 25

[Peer]
PublicKey = c2Vjb25kLXBlZXItbXVzdC1iZS1pZ25vcmVkLTAwMDAwMDA=
Endpoint = 192.0.2.99:1
`

func TestConfURIToAWG2(t *testing.T) {
	raw := "amneziawg://" + base64.StdEncoding.EncodeToString([]byte(testConf)) + "#%D0%9B%D0%B0%D1%82%D0%B2%D0%B8%D1%8F"
	got, err := ConfURIToAWG2(raw)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseAWG2URI(got)
	if err != nil {
		t.Fatalf("result is not a valid awg2:// link: %v", err)
	}
	checks := map[string][2]string{
		"host":     {p.Host, "192.0.2.10"},
		"port":     {p.Port, "41491"},
		"address":  {p.Address, "10.78.0.2/32"},
		"mtu":      {p.MTU, "1280"},
		"allowed":  {p.AllowedIPs, "0.0.0.0/0,::/0"},
		"keepal":   {p.PersistentKeepalive, "25"},
		"jc":       {p.Jc, "4"},
		"s3":       {p.S3, "48"},
		"h4":       {p.H4, "4"},
		"hpk":      {p.HeaderProtectionKey, "aGVhZGVyLXByb3RlY3Rpb24ta2V5LXRlc3Qtb25seS0w"},
		"padding":  {p.ContentPaddingAddition, "10-100"},
		"trailers": {p.RandomTrailers, "on"},
		"cookies":  {p.DisableCookies, "on"},
		"rekey":    {p.RekeyAfterTime, "100-120"},
		"attempts": {p.MaxHandshakeAttempts, "15-20"},
		"psk":      {p.PresharedKey, "cHJlc2hhcmVkLWtleS1mb3ItdGVzdHMtb25seS0wMDAwMDA="},
		"pub":      {p.PublicKey, "c2VydmVyLXB1YmxpYy1rZXktZm9yLXRlc3RzLW9ubHkwMDA="},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if u, _ := url.Parse(got); u.Fragment != "" {
		t.Errorf("name fragment leaked into the link: %q", u.Fragment)
	}
}

func TestConfURIToAWG2_URLSafeBase64(t *testing.T) {
	body := base64.RawURLEncoding.EncodeToString([]byte(testConf))
	if _, err := ConfURIToAWG2("amneziawg://" + body); err != nil {
		t.Fatal(err)
	}
}

func TestConfURIToAWG2_IPv6Endpoint(t *testing.T) {
	conf := "[Interface]\nPrivateKey = a\nAddress = 10.0.0.2/32\n[Peer]\nPublicKey = b\nEndpoint = [2001:db8::1]:51820\n"
	got, err := ConfURIToAWG2("amneziawg://" + base64.StdEncoding.EncodeToString([]byte(conf)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseAWG2URI(got)
	if err != nil {
		t.Fatal(err)
	}
	if p.Host != "2001:db8::1" || p.Port != "51820" {
		t.Fatalf("host/port = %q/%q", p.Host, p.Port)
	}
}

func TestConfURIToAWG2_Rejects(t *testing.T) {
	enc := func(s string) string { return "amneziawg://" + base64.StdEncoding.EncodeToString([]byte(s)) }
	for name, raw := range map[string]string{
		"other scheme": "awg2://192.0.2.1:1",
		"not base64":   "amneziawg://!!!",
		"no endpoint":  enc("[Interface]\nPrivateKey = a\n[Peer]\nPublicKey = b\n"),
		"no keys":      enc("[Interface]\nAddress = 10.0.0.2/32\n[Peer]\nEndpoint = 192.0.2.1:1\n"),
	} {
		if _, err := ConfURIToAWG2(raw); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
