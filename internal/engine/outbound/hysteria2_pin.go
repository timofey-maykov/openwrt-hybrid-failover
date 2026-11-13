package outbound

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	aTLS "github.com/sagernet/sing/common/tls"
)

// Hysteria pinSHA256 is a leaf certificate fingerprint, not an SPKI hash.
// A valid pin supplies the trust anchor for a self-signed certificate. Keep this
// check active even for insecure=1 and TLS session resumption.
func pinHysteriaCertificate(config aTLS.Config, rawURI string) error {
	u, err := url.Parse(rawURI)
	if err != nil {
		return fmt.Errorf("hysteria2: invalid URI")
	}
	values, present := u.Query()["pinSHA256"]
	if !present {
		return nil
	}
	if len(values) != 1 {
		return fmt.Errorf("hysteria2: expected one certificate fingerprint")
	}
	fingerprint := strings.ReplaceAll(strings.TrimSpace(values[0]), ":", "")
	pin, err := hex.DecodeString(fingerprint)
	if err != nil || len(pin) != sha256.Size {
		return fmt.Errorf("hysteria2: invalid certificate SHA-256 fingerprint")
	}
	cfg, err := config.Config()
	if err != nil {
		return fmt.Errorf("hysteria2: certificate pin requires standard TLS: %w", err)
	}
	previous := cfg.VerifyConnection
	cfg.InsecureSkipVerify = true // Trust the mandatory fingerprint below instead of the system CA store.
	cfg.VerifyConnection = func(state tls.ConnectionState) error {
		if len(state.PeerCertificates) == 0 {
			return fmt.Errorf("hysteria2: server certificate missing")
		}
		leaf := sha256.Sum256(state.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(pin, leaf[:]) != 1 {
			return fmt.Errorf("hysteria2: server certificate fingerprint mismatch")
		}
		if previous != nil {
			return previous(state)
		}
		return nil
	}
	return nil
}
