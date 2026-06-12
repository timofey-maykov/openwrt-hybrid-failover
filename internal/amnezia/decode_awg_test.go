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

func TestDecodeVPNURIAmneziaAWGLink(t *testing.T) {
	const raw = "vpn://AAABxnicjVC7bsMwDPwXzbFgu02TGOjQduvQHwgKQVWZhrAtCxLtPAz_e0gjQJYOvoHgkXcEeKNynSeLHmJS1X58UFUp23q4os3s6U-tlNRqVI1NZFh1wJkeu0RftgWWF7tS57rURc7q0EXi2fNusy2YugbBk8EgulxvdaHLxzhEHEwNF16-LccrH0gQB4gm9D93__tyiD-k-m78WA4xttTLK0-bF7nC6WEieaUGCMY2OEgi5ZqXn05ykKZFP78_9_bM_TpX0zR9r9QvJBcxEHYiSRdPRyB0GUGi7IBn6iOw7f-0pxuz1IYv"
	uri, err := DecodeVPNURI(raw)
	if err != nil {
		t.Fatalf("DecodeVPNURI: %v", err)
	}
	if !strings.HasPrefix(uri, "awg2://192.0.2.10:49781?") {
		t.Fatalf("uri=%q", uri)
	}
	desc, err := DescribeLink(raw)
	if err != nil {
		t.Fatalf("DescribeLink: %v", err)
	}
	if desc != "awg2 interface pawg5ebeb606" {
		t.Fatalf("desc=%q", desc)
	}
}
