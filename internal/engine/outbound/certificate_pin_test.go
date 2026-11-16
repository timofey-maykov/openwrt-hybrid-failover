package outbound

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const helloFragment = `{"tcp":[{"type":"fragment","settings":{"length":"10-20","packets":"tlshello","interval":"0"}}]}`

func TestPinnedTLSHandshakeWithEmptySNIAndFragment(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	good := sha256.Sum256(server.Certificate().Raw)
	for _, fp := range []string{"none", "firefox", "qq"} {
		for _, fragment := range []bool{false, true} {
			for _, valid := range []bool{false, true} {
				t.Run(fp+"/fragment="+boolText(fragment)+"/valid="+boolText(valid), func(t *testing.T) {
					pin := hex.EncodeToString(good[:])
					if !valid {
						pin = strings.Repeat("0", 64)
					}
					fields := map[string]any{"tls": map[string]any{"enabled": true, "insecure": true, "pinned_peer_cert_sha256": pin, "utls": map[string]any{"enabled": true, "fingerprint": fp}}}
					if fragment {
						fields["finalmask"] = helloFragment
					}
					// A leaf pin, rather than a guessed name or system CA, authenticates this peer.
					cfg, err := buildTLSConfig(fields, "wrong-name.invalid")
					if err != nil {
						t.Fatal(err)
					}
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
					if err != nil {
						t.Fatal(err)
					}
					defer raw.Close()
					conn, err := cfg.Clone().Client(raw)
					if err != nil {
						t.Fatal(err)
					}
					err = conn.HandshakeContext(ctx)
					if (err == nil) != valid {
						t.Fatalf("valid=%v error=%v", valid, err)
					}
				})
			}
		}
	}
}

func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func TestCertificatePinCannotTrustAnAppendedUnrelatedCertificate(t *testing.T) {
	a := selfSigned(t)
	b := selfSigned(t)
	leaf, _ := x509.ParseCertificate(a.Certificate[0])
	other, _ := x509.ParseCertificate(b.Certificate[0])
	pin := sha256.Sum256(other.Raw)
	if err := verifyCertificatePins([][]byte{pin[:]}, []*x509.Certificate{leaf, other}, "localhost"); err == nil {
		t.Fatal("unrelated appended certificate authenticated the peer")
	}
	if err := verifyCertificatePins([][]byte{pin[:]}, nil, ""); err == nil {
		t.Fatal("missing certificate accepted")
	}
}

func TestInvalidCertificatePinsFailBeforeDial(t *testing.T) {
	for _, pin := range []any{"", "bad", strings.Repeat("a", 63), strings.Repeat("a", 64) + "~", strings.Repeat("a", 64) + "," + strings.Repeat("b", 64), false} {
		if _, err := parseCertificatePins(pin); err == nil {
			t.Fatalf("invalid pin accepted: %v", pin)
		}
	}
	if pins, err := parseCertificatePins(strings.Repeat("A", 64) + "~" + strings.Repeat("b", 64)); err != nil || len(pins) != 2 {
		t.Fatal("valid alternative pins rejected")
	}
}

func TestCAPinVerifiesLeafChainAndName(t *testing.T) {
	a, b := selfSigned(t), selfSigned(t)
	leafTemplate, _ := x509.ParseCertificate(a.Certificate[0])
	caTemplate, _ := x509.ParseCertificate(b.Certificate[0])
	caTemplate.IsCA = true
	caTemplate.BasicConstraintsValid = true
	caTemplate.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caTemplate.PublicKey, b.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, _ := x509.ParseCertificate(caDER)
	leafTemplate.SerialNumber = big.NewInt(2)
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, leafTemplate.PublicKey, b.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(leafDER)
	forged, _ := x509.ParseCertificate(a.Certificate[0])
	pin := sha256.Sum256(ca.Raw)
	for _, tc := range []struct {
		name  string
		cert  *x509.Certificate
		host  string
		valid bool
	}{
		{"valid chain", leaf, "localhost", true}, {"wrong host", leaf, "wrong.invalid", false}, {"empty host", leaf, "", false}, {"forged leaf with pinned CA appended", forged, "localhost", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyCertificatePins([][]byte{pin[:]}, []*x509.Certificate{tc.cert, ca}, tc.host)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
