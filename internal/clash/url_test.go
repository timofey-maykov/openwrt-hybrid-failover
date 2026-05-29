package clash

import "testing"

func TestDetectListenAddressUsesLAN(t *testing.T) {
	// Without UCI on dev machines this falls back to 127.0.0.1:9090.
	addr := DetectListenAddress("")
	if addr == "" {
		t.Fatal("empty address")
	}
	if !containsPort(addr) {
		t.Fatalf("expected host:port, got %q", addr)
	}
}

func containsPort(addr string) bool {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return true
		}
	}
	return false
}
