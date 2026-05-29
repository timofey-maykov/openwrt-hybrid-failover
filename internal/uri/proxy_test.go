package uri_test

import (
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

func TestParseVLESS(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@example.com:443?encryption=none&security=reality&sni=example.com&fp=chrome&pbk=VDx8FnyKEJntMxyrVqRXJqfdhnnz9tNTsQr064RBTWU&sid=abcd&type=tcp"
	ob, err := uri.ParseProxy(raw, "glob-1-out", false)
	if err != nil {
		t.Fatal(err)
	}
	if ob.Fields["type"] != "vless" {
		t.Fatalf("type=%v", ob.Fields["type"])
	}
	if ob.Fields["tag"] != "glob-1-out" {
		t.Fatalf("tag=%v", ob.Fields["tag"])
	}
}

func TestParseSocks5(t *testing.T) {
	raw := "socks5://user:pass@127.0.0.1:1080"
	ob, err := uri.ParseProxy(raw, "test-out", true)
	if err != nil {
		t.Fatal(err)
	}
	if ob.Fields["type"] != "socks" {
		t.Fatalf("type=%v", ob.Fields["type"])
	}
}
