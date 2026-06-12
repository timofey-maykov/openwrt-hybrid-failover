package lifecycle

import "testing"

func TestParseAWGShowPeer(t *testing.T) {
	text := `interface: pawgtest0001
  public key: BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=
  private key: (hidden)
peer: CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=
  endpoint: 192.0.2.10:38889
  latest handshake: 31 seconds ago`

	endpoint, publicKey, ok := parseAWGShowPeer(text)
	if !ok {
		t.Fatal("expected peer parse ok")
	}
	if endpoint != "192.0.2.10:38889" {
		t.Fatalf("endpoint=%q", endpoint)
	}
	if publicKey != "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC=" {
		t.Fatalf("publicKey=%q", publicKey)
	}
}

func TestParseAWGShowPeerMissing(t *testing.T) {
	if _, _, ok := parseAWGShowPeer("interface: pawg\n  public key: x"); ok {
		t.Fatal("expected missing peer")
	}
}
