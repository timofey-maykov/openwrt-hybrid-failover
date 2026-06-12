package uri_test

import (
	"encoding/json"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

func TestParseHysteria2StandardURI(t *testing.T) {
	raw := "hysteria2://secret@example.com:443?sni=real.example.com&insecure=1&obfs=salamander&obfs-password=obfspass"
	ob, err := uri.ParseProxy(raw, "main-1-out", false)
	if err != nil {
		t.Fatal(err)
	}
	if ob.Fields["type"] != "hysteria2" {
		t.Fatalf("type=%v", ob.Fields["type"])
	}
	if ob.Fields["password"] != "secret" {
		t.Fatalf("password=%v", ob.Fields["password"])
	}
	tls, ok := ob.Fields["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls=%T", ob.Fields["tls"])
	}
	if tls["enabled"] != true {
		t.Fatalf("tls.enabled=%v", tls["enabled"])
	}
	if tls["server_name"] != "real.example.com" {
		t.Fatalf("server_name=%v", tls["server_name"])
	}
	if tls["insecure"] != true {
		t.Fatalf("insecure=%v", tls["insecure"])
	}
	obfs, ok := ob.Fields["obfs"].(map[string]any)
	if !ok || obfs["type"] != "salamander" || obfs["password"] != "obfspass" {
		t.Fatalf("obfs=%v", ob.Fields["obfs"])
	}
	if _, ok := ob.Fields["transport"]; ok {
		t.Fatalf("unexpected transport: %v", ob.Fields["transport"])
	}
}

func TestParseHysteria2Userpass(t *testing.T) {
	raw := "hy2://user:pass@127.0.0.1:8443"
	ob, err := uri.ParseProxy(raw, "hy-out", false)
	if err != nil {
		t.Fatal(err)
	}
	if ob.Fields["password"] != "user:pass" {
		t.Fatalf("password=%v", ob.Fields["password"])
	}
	tls := ob.Fields["tls"].(map[string]any)
	if tls["server_name"] != "127.0.0.1" {
		t.Fatalf("server_name=%v", tls["server_name"])
	}
}

func TestParseHysteria2BandwidthAliases(t *testing.T) {
	raw := "hysteria2://pw@host.test?up=20&down=50"
	ob, err := uri.ParseProxy(raw, "hy-out", false)
	if err != nil {
		t.Fatal(err)
	}
	if ob.Fields["up_mbps"] != 20 || ob.Fields["down_mbps"] != 50 {
		t.Fatalf("bw=%v %v", ob.Fields["up_mbps"], ob.Fields["down_mbps"])
	}
}

func TestParseHysteria2JSONForSingbox(t *testing.T) {
	raw := "hysteria2://pw@host.test:443?sni=host.test"
	ob, err := uri.ParseProxy(raw, "hy-out", false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ob.Fields)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatalf("invalid json: %s", data)
	}
}
