package outbound

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHysteriaPinnedSelfSignedTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer server.Close()
	leaf := server.Certificate()
	sum := sha256.Sum256(leaf.Raw)
	pin := hex.EncodeToString(sum[:])
	for _, tc := range []struct {
		name, pin, query string
		wantErr          bool
	}{
		{"pin-only existing subscription", pin, "", false},
		{"standard insecure plus pin", pin, "&insecure=1", false},
		{"wrong pin even with insecure", strings.Repeat("00", 32), "&insecure=1", true},
		{"untrusted CA without pin", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := "hysteria2://test@127.0.0.1:443?" + strings.TrimPrefix(tc.query, "&")
			if tc.pin != "" {
				raw += "&pinSHA256=" + tc.pin
			}
			cfg, err := buildTLSConfig(map[string]any{"tls": map[string]any{"enabled": true, "server_name": "127.0.0.1"}}, "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			if err := pinHysteriaCertificate(cfg, raw); err != nil {
				t.Fatal(err)
			}
			conn, err := net.DialTimeout("tcp", server.Listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			secured, err := dialTLS(ctx, conn, cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("TLS pin verification: %v", err)
			}
			if secured != nil {
				_ = secured.Close()
			}
		})
	}
}

func TestHysteriaPinRejectsMalformedAndCACertificateInsteadOfLeaf(t *testing.T) {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	defer server.Close()
	sum := sha256.Sum256(server.Certificate().Raw)
	pin := hex.EncodeToString(sum[:])
	for _, value := range []string{"", "bad", strings.Repeat("a", 62), pin + "&pinSHA256=" + pin} {
		cfg := newSTDConfig(tlsOptions{enabled: true})
		if err := pinHysteriaCertificate(cfg, "hysteria2://test@host?insecure=1&pinSHA256="+value); err == nil {
			t.Fatal("malformed pin accepted")
		}
	}
	cfg := newSTDConfig(tlsOptions{enabled: true})
	colonPin := strings.ToUpper(strings.Join(splitPinPairs(pin), ":"))
	if err := pinHysteriaCertificate(cfg, "hysteria2://test@host?pinSHA256="+colonPin); err != nil {
		t.Fatal(err)
	}
	std, _ := cfg.Config()
	if err := std.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{{Raw: []byte("forged leaf")}, server.Certificate()}}); err == nil {
		t.Fatal("pin incorrectly matched another certificate in chain")
	}
	if err := std.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Fatal("missing certificate accepted")
	}
	clone, _ := cfg.Clone().Config()
	if err := clone.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{server.Certificate()}}); err != nil {
		t.Fatal("cloned/session verifier lost pin")
	}
}
func splitPinPairs(value string) []string {
	var out []string
	for i := 0; i < len(value); i += 2 {
		out = append(out, value[i:i+2])
	}
	return out
}
