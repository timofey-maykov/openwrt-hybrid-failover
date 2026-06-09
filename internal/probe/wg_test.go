package probe

import (
	"testing"
	"time"
)

func TestParseLatestHandshake(t *testing.T) {
	out := []byte("O7XKDAGkXPDUDv0rNPKzGb7/iA+sJFMTBHxUTHQJhh0=\t1780823449\n")
	last, ok := parseLatestHandshake(out)
	if !ok {
		t.Fatal("expected handshake")
	}
	if last.Unix() != 1780823449 {
		t.Fatalf("timestamp = %d", last.Unix())
	}
}

func TestParseLatestHandshakeEmpty(t *testing.T) {
	if _, ok := parseLatestHandshake([]byte("peer\t0\n")); ok {
		t.Fatal("expected no handshake for ts=0")
	}
}

func TestFormatDurationShort(t *testing.T) {
	if got := formatDurationShort(90 * time.Second); got != "1m" {
		t.Fatalf("got %q", got)
	}
}
