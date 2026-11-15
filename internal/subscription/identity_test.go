package subscription

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/uci"
)

func TestRouterHWIDMatchesNetShiftAcrossRestarts(t *testing.T) {
	mac, model := "02:00:00:00:00:01", "Test OpenWrt Router"
	h := md5.Sum([]byte(mac + "-" + model))
	raw := hex.EncodeToString(h[:])[:16]
	want := raw[:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:]
	if got := routerHWID(mac, model); got != want || got != routerHWID(" "+mac+"\n", model+"\n") {
		t.Fatal("unstable or incompatible router identity")
	}
	if routerHWID("02:00:00:00:00:02", model) == want {
		t.Fatal("different routers share an identity")
	}
}

func TestIdentitySettingsAndHeaderValidation(t *testing.T) {
	sec := &uci.Section{Options: map[string]string{"subscription_user_agent": "Custom/1.0", "subscription_hwid": "test-device", "subscription_device_os": "Android", "subscription_os_version": "14", "subscription_device_model": "Test Phone"}}
	id, err := IdentityFromSettings(sec)
	if err != nil {
		t.Fatal(err)
	}
	h := id.headers(id.userAgents()[0])
	for key, want := range map[string]string{"User-Agent": "Custom/1.0", "X-HWID": "test-device", "X-Device-OS": "Android", "X-Ver-OS": "14", "X-Device-Model": "Test Phone", "X-Device-Locale": "EN"} {
		if h.Get(key) != want {
			t.Errorf("missing identity header %s", key)
		}
	}
	if len(id.userAgents()) != 1 {
		t.Fatal("explicit user agent must disable negotiation")
	}
	sec.Options["subscription_hwid"] = "test\r\nAuthorization: SECRET"
	if _, err := IdentityFromSettings(sec); err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("invalid identity must fail without echoing its value")
	}
}

func TestClientNegotiationRejectsNoticeAndKeepsSameHWID(t *testing.T) {
	var agents []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.UserAgent())
		if r.Header.Get("X-HWID") != "test-router" || r.Header.Get("X-Device-OS") != "OpenWrt Linux" {
			t.Error("identity missing during negotiation")
		}
		if r.UserAgent() == "v2rayN" {
			_, _ = io.WriteString(w, "vless://00000000-0000-0000-0000-000000000000@192.0.2.1:443#unsupported%20client")
		} else {
			_, _ = io.WriteString(w, `[{"remarks":"Working","outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"192.0.2.20","port":443,"users":[{"id":"11111111-1111-4111-8111-111111111111","encryption":"none"}]}]},"streamSettings":{"security":"tls","tlsSettings":{"serverName":"example.com"}}}]}]`)
		}
	}))
	defer server.Close()
	f := NewFetcher()
	f.Identity = ClientIdentity{HWID: "test-router", DeviceOS: "OpenWrt Linux"}
	f.FallbackClients = func(context.Context) []*http.Client {
		t.Fatal("successful negotiation must not try tunnels")
		return nil
	}
	res, err := f.Fetch([]string{server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(agents, []string{"v2rayN", "Happ/1.0.0"}) || len(res.Links) != 1 || res.Links[0].Name != "Working" {
		t.Fatalf("agents=%v links=%d", agents, len(res.Links))
	}
}

func TestNetworkFailureDoesNotProbeEveryUserAgent(t *testing.T) {
	attempts := 0
	f := NewFetcher()
	f.FallbackClients = nil
	f.HTTP = &http.Client{Transport: responseTransport(func(*http.Request) (*http.Response, error) { attempts++; return nil, io.ErrUnexpectedEOF })}
	_, err := f.Fetch([]string{"https://blocked.example/private-token"})
	if err == nil || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}

func TestTunnelRetryPreservesClientHeaders(t *testing.T) {
	attempts := 0
	f := NewFetcher()
	f.Identity = ClientIdentity{HWID: "test-router", UserAgent: "Custom/1.0", DeviceOS: "OpenWrt Linux"}
	f.HTTP = &http.Client{Transport: responseTransport(func(*http.Request) (*http.Response, error) { return nil, io.ErrUnexpectedEOF })}
	f.FallbackClients = func(context.Context) []*http.Client {
		return []*http.Client{{Transport: responseTransport(func(r *http.Request) (*http.Response, error) {
			attempts++
			if r.UserAgent() != "Custom/1.0" || r.Header.Get("X-HWID") != "test-router" {
				t.Error("lost identity on tunnel retry")
			}
			return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("hy2://password@192.0.2.20:443"))}, nil
		})}}
	}
	if _, err := f.Fetch([]string{"https://blocked.example/private-token"}); err != nil || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
}
