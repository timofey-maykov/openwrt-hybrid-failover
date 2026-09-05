package amnezia

import (
	"strings"
	"testing"
)

func TestExtractProxyURIAmneziaAWG(t *testing.T) {
	doc := map[string]any{
		"containers": []any{
			map[string]any{
				"container": "amnezia-awg",
				"awg": map[string]any{
					"last_config": map[string]any{
						"hostName":              "192.0.2.10",
						"port":                  "49781",
						"client_ip":             "10.8.1.2",
						"client_priv_key":       "client-priv",
						"server_pub_key":        "server-pub",
						"psk_key":               "psk",
						"mtu":                   "1376",
						"persistent_keep_alive": "25",
						"Jc":                    "4",
						"Jmin":                  "10",
						"Jmax":                  "50",
					},
				},
			},
		},
	}
	uri, err := extractProxyURI(doc)
	if err != nil {
		t.Fatalf("extractProxyURI: %v", err)
	}
	if !strings.HasPrefix(uri, "awg2://192.0.2.10:49781?") {
		t.Fatalf("uri=%q", uri)
	}
	if !strings.Contains(uri, "private_key=client-priv") {
		t.Fatalf("missing private_key in %q", uri)
	}
}

func TestExtractProxyURIAmneziaAWG31(t *testing.T) {
	doc := map[string]any{
		"containers": []any{
			map[string]any{
				"container": "amnezia-awg2",
				"awg": map[string]any{
					"protocol_version": "3.1",
					"last_config": map[string]any{
						"hostName":               "192.0.2.10",
						"port":                   "41490",
						"client_ip":              "10.8.1.2",
						"client_priv_key":        "client-priv",
						"server_pub_key":         "server-pub",
						"psk_key":                "psk",
						"mtu":                    "1376",
						"persistent_keep_alive":  "25-35",
						"Jc":                     "4",
						"Jmin":                   "10",
						"Jmax":                   "50",
						"S1":                     "67",
						"S2":                     "66",
						"S3":                     "48",
						"S4":                     "12",
						"H1":                     "1",
						"H2":                     "2",
						"H3":                     "3",
						"H4":                     "4",
						"HeaderProtectionKey":    "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD=",
						"ContentPaddingAddition": "10-100",
						"RandomTrailers":         "on",
						"DisableCookies":         "on",
						"RekeyAfterTime":         "100-120",
						"RekeyTimeout":           "3-7",
						"RejectAfterTime":        "150-180",
						"KeepaliveTimeout":       "5-15",
						"MaxHandshakeAttempts":   "15-20",
					},
				},
			},
		},
	}
	uri, err := extractProxyURI(doc)
	if err != nil {
		t.Fatalf("extractProxyURI: %v", err)
	}
	p, err := ParseAWG2URI(uri)
	if err != nil {
		t.Fatalf("ParseAWG2URI: %v uri=%q", err, uri)
	}
	if p.Host != "192.0.2.10" || p.Port != "41490" {
		t.Fatalf("endpoint=%s:%s", p.Host, p.Port)
	}
	if p.PersistentKeepalive != "25-35" {
		t.Fatalf("persistent_keepalive=%q", p.PersistentKeepalive)
	}
	if p.HeaderProtectionKey != "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD=" {
		t.Fatalf("HeaderProtectionKey=%q", p.HeaderProtectionKey)
	}
	if p.ContentPaddingAddition != "10-100" || p.RandomTrailers != "on" || p.DisableCookies != "on" {
		t.Fatalf("padding/trailers/cookies: %q %q %q", p.ContentPaddingAddition, p.RandomTrailers, p.DisableCookies)
	}
	if p.RekeyAfterTime != "100-120" || p.KeepaliveTimeout != "5-15" || p.MaxHandshakeAttempts != "15-20" {
		t.Fatalf("timers: rekey=%q keepalive=%q attempts=%q", p.RekeyAfterTime, p.KeepaliveTimeout, p.MaxHandshakeAttempts)
	}
	if p.S4 != "12" {
		t.Fatalf("s4=%q", p.S4)
	}
}

func TestParseAWG2URIAWG31Query(t *testing.T) {
	raw := "awg2://192.0.2.10:41490?address=10.8.1.2%2F32&private_key=k&public_key=p" +
		"&header_protection_key=DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD%3D" +
		"&content_padding_addition=10-100&random_trailers=on&disable_cookies=on" +
		"&rekey_after_time=100-120&rekey_timeout=3-7&reject_after_time=150-180" +
		"&keepalive_timeout=5-15&max_handshake_attempts=15-20&persistent_keepalive=25-35"
	p, err := ParseAWG2URI(raw)
	if err != nil {
		t.Fatalf("ParseAWG2URI: %v", err)
	}
	if p.HeaderProtectionKey != "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD=" {
		t.Fatalf("HeaderProtectionKey=%q", p.HeaderProtectionKey)
	}
	if p.ContentPaddingAddition != "10-100" || p.RandomTrailers != "on" || p.DisableCookies != "on" {
		t.Fatalf("got padding=%q trailers=%q cookies=%q", p.ContentPaddingAddition, p.RandomTrailers, p.DisableCookies)
	}
	if p.PersistentKeepalive != "25-35" {
		t.Fatalf("persistent_keepalive=%q", p.PersistentKeepalive)
	}
}

func TestDecodeVPNURIAmneziaAWGLink(t *testing.T) {
	// Synthetic Amnezia export (TEST-NET-1). Do not embed real vpn:// dumps in tests.
	const raw = "vpn://AAABxnicjVC7bsMwDPwXzbFgu02TGOjQduvQHwgKQVWZhrAtCxLtPAz_e0gjQJYOvoHgkXcEeKNynSeLHmJS1X58UFUp23q4os3s6U-tlNRqVI1NZFh1wJkeu0RftgWWF7tS57rURc7q0EXi2fNusy2YugbBk8EgulxvdaHLxzhEHEwNF16-LccrH0gQB4gm9D93__tyiD-k-m78WA4xttTLK0-bF7nC6WEieaUGCMY2OEgi5ZqXn05ykKZFP78_9_bM_TpX0zR9r9QvJBcxEHYiSRdPRyB0GUGi7IBn6iOw7f-0pxuz1IYv"
	uri, err := DecodeVPNURI(raw)
	if err != nil {
		t.Fatalf("DecodeVPNURI: %v", err)
	}
	if !strings.HasPrefix(uri, "awg2://192.0.2.10:49781?") {
		t.Fatalf("uri=%q", uri)
	}
	if !strings.Contains(uri, "private_key=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA%3D") {
		t.Fatalf("missing synthetic private_key in %q", uri)
	}
	desc, err := DescribeLink(raw)
	if err != nil {
		t.Fatalf("DescribeLink: %v", err)
	}
	if desc != "awg2 interface pawg5ebeb606" {
		t.Fatalf("desc=%q", desc)
	}
}
