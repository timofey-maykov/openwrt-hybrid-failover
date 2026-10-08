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

func tlsOf(t *testing.T, raw string) map[string]any {
	t.Helper()
	ob, err := uri.ParseProxy(raw, "t-out", false)
	if err != nil {
		t.Fatal(err)
	}
	tls, _ := ob.Fields["tls"].(map[string]any)
	if tls == nil {
		t.Fatalf("no tls block in %v", ob.Fields)
	}
	return tls
}

const vlessID = "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&sni=front.example.com"

func TestVLESSTLSOptionsFromTheLink(t *testing.T) {
	for _, flag := range []string{"allowInsecure=1", "insecure=1", "allow_insecure=true"} {
		if tls := tlsOf(t, vlessID+"&"+flag); tls["insecure"] != true {
			t.Errorf("%s was ignored: %v", flag, tls)
		}
	}
	if tls := tlsOf(t, vlessID); tls["insecure"] != nil {
		t.Errorf("certificate checks are off without being asked: %v", tls)
	}
	tls := tlsOf(t, vlessID+"&alpn=h2,http/1.1")
	if list, _ := tls["alpn"].([]string); len(list) != 2 || list[0] != "h2" || list[1] != "http/1.1" {
		t.Errorf("alpn: %v", tls["alpn"])
	}
	if tls := tlsOf(t, "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&peer=old.example.com"); tls["server_name"] != "old.example.com" {
		t.Errorf("peer is the older name for sni: %v", tls)
	}
}

func TestVLESSTransportsFromTheLink(t *testing.T) {
	ws, err := uri.ParseProxy(vlessID+"&type=ws&path=%2Fp%3Fed%3D2048&host=cdn.example.com", "t-out", false)
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := ws.Fields["transport"].(map[string]any)
	if tr["type"] != "ws" || tr["path"] != "/p?ed=2048" {
		t.Fatalf("ws: %v", tr)
	}
	if h, _ := tr["headers"].(map[string]string); h["Host"] != "cdn.example.com" {
		t.Fatalf("ws host: %v", tr["headers"])
	}
	g, err := uri.ParseProxy(vlessID+"&type=grpc&serviceName=svc&authority=a.example.com", "t-out", false)
	if err != nil {
		t.Fatal(err)
	}
	tr, _ = g.Fields["transport"].(map[string]any)
	if tr["type"] != "grpc" || tr["service_name"] != "svc" || tr["authority"] != "a.example.com" {
		t.Fatalf("grpc: %v", tr)
	}
}
