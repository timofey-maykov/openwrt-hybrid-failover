package outbound

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	utls "github.com/metacubex/utls"
)

var fingerprintNames = []string{"chrome", "firefox", "safari", "ios", "android", "edge", "qq", "360", "chrome120", "firefox120", "safari16", "chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk", "random", "randomized", "randomizedalpn", "randomizednoalpn"}

func TestFingerprintTLSHandshake(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	for _, name := range fingerprintNames {
		t.Run(name, func(t *testing.T) {
			cfg, err := newUTLSConfig(tlsOptions{serverName: "example.com", fingerprint: name})
			if err != nil {
				t.Fatal(err)
			}
			cfg.(*utlsConfig).cfg.RootCAs = roots
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			conn, err := cfg.Client(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := conn.HandshakeContext(ctx); err != nil {
				t.Fatal(err)
			}
			if len(conn.(*utlsConnWrap).UConn.ConnectionState().VerifiedChains) == 0 {
				t.Fatal("certificate was not verified")
			}
		})
	}
}

func TestFingerprintRealityKeyShares(t *testing.T) {
	for _, name := range fingerprintNames {
		t.Run(name, func(t *testing.T) {
			id, err := utlsFingerprint(name)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := newRealityConfig(tlsOptions{serverName: "example.com", fingerprint: name})
			if name == "android" || name == "360" {
				if err == nil || !strings.Contains(err.Error(), "TLS 1.3") {
					t.Fatalf("legacy REALITY profile error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_ = cfg
			conn := utls.UClient(nil, &utls.Config{ServerName: "example.com", OmitEmptyPsk: true}, id)
			if err := conn.BuildHandshakeState(); err != nil {
				t.Fatal(err)
			}
			if realityECDHE(conn.HandshakeState.State13.KeyShareKeys) == nil {
				t.Fatal("fingerprint has no Reality X25519 key share")
			}
		})
	}
}

func TestTLSFingerprintNoneAndDisabled(t *testing.T) {
	for _, utlsFields := range []map[string]any{{"enabled": true, "fingerprint": "none"}, {"enabled": false, "fingerprint": "unknown"}} {
		cfg, err := buildTLSConfig(map[string]any{"tls": map[string]any{"enabled": true, "utls": utlsFields}}, "example.com")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := cfg.(*stdTLSConfig); !ok {
			t.Fatal("expected verified standard TLS")
		}
	}
	if _, err := utlsFingerprint("unknown"); err == nil {
		t.Fatal("unknown fingerprint was silently replaced")
	}
}
