package outbound

import (
	"net"
	"testing"

	utls "github.com/metacubex/utls"
)

func TestChromeFingerprintKeepsMLKEMKeyShare(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "www.live.com", InsecureSkipVerify: true}, utls.HelloChrome_Auto)
	if err := uc.BuildHandshakeState(); err != nil {
		t.Fatal(err)
	}
	hasMLKEM := false
	for _, ext := range uc.Extensions {
		ks, ok := ext.(*utls.KeyShareExtension)
		if !ok {
			continue
		}
		for _, share := range ks.KeyShares {
			if share.Group == utls.X25519MLKEM768 {
				hasMLKEM = true
			}
		}
	}
	if !hasMLKEM {
		t.Fatal("chrome fingerprint must offer X25519MLKEM768 for Xray Reality ≥ 26.9.8")
	}
	if realityECDHE(uc.HandshakeState.State13.KeyShareKeys) == nil {
		t.Fatal("chrome fingerprint must expose an X25519 key for Reality SessionId auth")
	}
}

func TestRealityECDHEFallsBackToMlkemEcdhe(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	uc := utls.UClient(c1, &utls.Config{ServerName: "www.live.com", InsecureSkipVerify: true}, utls.HelloChrome_133)
	if err := uc.BuildHandshakeState(); err != nil {
		t.Fatal(err)
	}
	keys := uc.HandshakeState.State13.KeyShareKeys
	if keys == nil {
		t.Fatal("no key shares")
	}
	// Chrome_133 often fills MlkemEcdhe and may leave Ecdhe nil when MLKEM is first.
	got := realityECDHE(keys)
	if got == nil {
		t.Fatal("expected MlkemEcdhe or Ecdhe")
	}
	if keys.Ecdhe == nil && keys.MlkemEcdhe == nil {
		t.Fatal("both ecdhe keys empty")
	}
	if keys.Ecdhe != nil && got != keys.Ecdhe {
		t.Fatal("prefer classical Ecdhe when present")
	}
	if keys.Ecdhe == nil && got != keys.MlkemEcdhe {
		t.Fatal("fall back to MlkemEcdhe")
	}
}
