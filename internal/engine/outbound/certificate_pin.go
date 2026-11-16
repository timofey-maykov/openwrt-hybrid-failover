package outbound

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
)

func parseCertificatePins(value any) ([][]byte, error) {
	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("pcs: certificate SHA-256 pin is required")
	}
	if strings.Count(text, "~") >= 64 {
		return nil, fmt.Errorf("pcs: too many certificate pins")
	}
	var pins [][]byte
	for _, entry := range strings.Split(text, "~") {
		pin, err := hex.DecodeString(strings.TrimSpace(entry))
		if err != nil || len(pin) != sha256.Size {
			return nil, fmt.Errorf("pcs: invalid certificate SHA-256 pin")
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

// PCS authenticates a leaf certificate directly, or a CA only after verifying
// the leaf's chain and server name against that CA. Matching an arbitrary
// appended certificate is never sufficient to authenticate a connection.
func verifyCertificatePins(pins [][]byte, certs []*x509.Certificate, serverName string) error {
	if len(certs) == 0 {
		return fmt.Errorf("pcs: server certificate missing")
	}
	matches := func(cert *x509.Certificate) bool {
		sum := sha256.Sum256(cert.Raw)
		for _, pin := range pins {
			if subtle.ConstantTimeCompare(pin, sum[:]) == 1 {
				return true
			}
		}
		return false
	}
	if matches(certs[0]) {
		return nil
	}
	for _, ca := range certs[1:] {
		if !ca.IsCA || !matches(ca) {
			continue
		}
		if serverName == "" {
			continue
		}
		roots := x509.NewCertPool()
		roots.AddCert(ca)
		intermediates := x509.NewCertPool()
		for _, cert := range certs[1:] {
			intermediates.AddCert(cert)
		}
		if _, err := certs[0].Verify(x509.VerifyOptions{Roots: roots, Intermediates: intermediates, DNSName: serverName}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("pcs: server certificate does not match a trusted pin")
}
