package amnezia

import (
	"os"
	"strings"
	"testing"
)

func TestDecodeVPNURIFromFile(t *testing.T) {
	path := os.Getenv("VPN_URI_FILE")
	if path == "" {
		t.Skip("set VPN_URI_FILE")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	uri, err := DecodeVPNURI(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("DecodeVPNURI: %v", err)
	}
	if !strings.HasPrefix(uri, "awg2://") && !strings.HasPrefix(uri, "vless://") {
		t.Fatalf("unexpected scheme: %s", uri[:min(20, len(uri))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
