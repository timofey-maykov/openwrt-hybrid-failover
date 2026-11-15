package subscription

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uri"
)

func TestSubscriptionBase64VariantsAndBOM(t *testing.T) {
	raw := "hy2://password@192.0.2.20:443#Тест 🚀\nvless://11111111-1111-4111-8111-111111111111@192.0.2.21:443?security=tls#Test"
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		b := enc.EncodeToString([]byte(raw))
		b = "\ufeff \n" + b[:12] + " \r\n" + b[12:] + "\n"
		entries, err := parseSubscription([]byte(b))
		if err != nil || len(entries) != 2 {
			t.Fatalf("entries=%d error=%v", len(entries), err)
		}
	}
}

func TestStructuredVLESSPreservesRealityAndTransport(t *testing.T) {
	for name, body := range map[string]string{
		"singbox": `{"outbounds":[{"type":"direct","tag":"direct"},{"type":"vless","tag":"Test","server":"2001:db8::1","server_port":443,"uuid":"11111111-1111-4111-8111-111111111111","tls":{"enabled":true,"server_name":"example.com","reality":{"enabled":true,"public_key":"TEST-PUBLIC-KEY","short_id":"abcd"},"utls":{"enabled":true,"fingerprint":"chrome"}},"transport":{"type":"grpc","service_name":"service"}}]}`,
		"xray":    `[{"remarks":"Test","outbounds":[{"protocol":"freedom"},{"protocol":"vless","settings":{"vnext":[{"address":"2001:db8::1","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111","encryption":"none"}]}]},"streamSettings":{"network":"grpc","security":"reality","realitySettings":{"serverName":"example.com","publicKey":"TEST-PUBLIC-KEY","shortId":"abcd","fingerprint":"chrome"},"grpcSettings":{"serviceName":"service"}}}]}]`,
		"clash":   "mixed-port: 7890\nproxies:\n  - name: Test\n    type: vless\n    server: 2001:db8::1\n    port: 443\n    uuid: 11111111-1111-4111-8111-111111111111\n    tls: true\n    servername: example.com\n    client-fingerprint: chrome\n    reality-opts:\n      public-key: TEST-PUBLIC-KEY\n      short-id: abcd\n    network: grpc\n    grpc-opts:\n      grpc-service-name: service\n",
	} {
		t.Run(name, func(t *testing.T) {
			entries, err := parseSubscription([]byte(body))
			if err != nil || len(entries) != 1 {
				t.Fatalf("entries=%d error=%v", len(entries), err)
			}
			link, _ := url.Parse(entries[0])
			q := link.Query()
			for key, want := range map[string]string{"security": "reality", "pbk": "TEST-PUBLIC-KEY", "sid": "abcd", "sni": "example.com", "fp": "chrome", "type": "grpc", "serviceName": "service"} {
				if q.Get(key) != want {
					t.Errorf("%s lost: %q", key, q.Get(key))
				}
			}
			if link.Hostname() != "2001:db8::1" || link.Port() != "443" || link.Fragment != "Test" {
				t.Fatal("lost IPv6 endpoint or server name")
			}
			if _, err := uri.ParseProxy(entries[0], "test", false); err != nil {
				t.Fatalf("native parser rejected converted server: %v", err)
			}
		})
	}
}

func TestStructuredSecretsAreEscapedAndTLSRetained(t *testing.T) {
	for _, body := range []string{
		`{"outbounds":[{"type":"trojan","tag":"Node & 1","server":"192.0.2.20","server_port":443,"password":"a:@/?#%&","tls":{"enabled":true,"server_name":"example.com","alpn":["h2","http/1.1"]},"transport":{"type":"ws","path":"/a?b=1&c=2","headers":{"Host":"example.com"}}}]}`,
		"proxies:\n - name: Node & 1\n   type: trojan\n   server: 192.0.2.20\n   port: 443\n   password: 'a:@/?#%&'\n   sni: example.com\n   alpn: [h2, http/1.1]\n   network: ws\n   ws-opts:\n     path: '/a?b=1&c=2'\n     headers: {Host: example.com}\n",
	} {
		entries, err := parseSubscription([]byte(body))
		if err != nil || len(entries) != 1 {
			t.Fatalf("entries=%d error=%v", len(entries), err)
		}
		p, err := uri.ParseProxy(entries[0], "test", false)
		if err != nil {
			t.Fatal(err)
		}
		if p.Fields["password"] != "a:@/?#%&" {
			t.Fatal("credential changed during conversion")
		}
		tls := object(p.Fields["tls"])
		if !enabled(tls["enabled"]) || enabled(tls["insecure"]) {
			t.Fatal("TLS verification changed")
		}
		tr := object(p.Fields["transport"])
		if textValue(tr, "path") != "/a?b=1&c=2" {
			t.Fatal("transport path changed")
		}
	}
}

func TestUnsupportedSecurityFeaturesAreNotSilentlyRemoved(t *testing.T) {
	for _, body := range []string{
		`{"outbounds":[{"type":"vless","server":"192.0.2.20","server_port":443,"uuid":"test","tls":{"enabled":true,"certificate_public_key_sha256":["pin"]}}]}`,
		`{"outbounds":[{"type":"vless","server":"192.0.2.20","server_port":443,"uuid":"test","transport":{"type":"ws","headers":{"Authorization":"SECRET"}}}]}`,
		`{"outbounds":[{"type":"vless","server":"192.0.2.20","server_port":443,"uuid":"test","transport":{"type":"xhttp","mode":"packet-up"}}]}`,
		`{"outbounds":[{"type":"shadowsocks","server":"192.0.2.20","server_port":443,"method":"aes-128-gcm","password":"secret","tls":{"enabled":true}}]}`,
	} {
		entries, err := parseSubscription([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatal("unsupported option was silently removed")
		}
	}
	if _, err := parseSubscription([]byte(`{"outbounds": [SECRET`)); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("invalid JSON error must not echo credentials")
	}
}

func TestXrayDoesNotDiscardChainedProxyOrAuthorization(t *testing.T) {
	for _, extra := range []string{`"proxySettings":{"tag":"hop"},`, `"streamSettings":{"network":"xhttp","xhttpSettings":{"host":"example.com","headers":{"Authorization":"SECRET"}}},`} {
		body := `{"outbounds":[{` + extra + `"protocol":"vless","settings":{"vnext":[{"address":"192.0.2.1","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111"}]}]}}]}`
		entries, err := parseSubscription([]byte(body))
		if err != nil || len(entries) != 0 {
			t.Fatalf("unsupported proxy constraint discarded: entries=%d error=%v", len(entries), err)
		}
	}
}
