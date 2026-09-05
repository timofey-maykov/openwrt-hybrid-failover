package probe

import (
	"strings"
	"testing"
	"time"
)

func TestOverlayAWG2Health(t *testing.T) {
	cases := []struct {
		name      string
		httpDelay int
		hs        AWGHandshake
		wantOK    bool
		wantDelay int
		wantSub   string
	}{
		{
			name:      "http urltest ok keeps delay",
			httpDelay: 900,
			hs:        AWGHandshake{Present: true, Fresh: true},
			wantOK:    true,
			wantDelay: 900,
		},
		{
			name:    "fresh handshake without http",
			hs:      AWGHandshake{Present: true, Fresh: true},
			wantOK:  true,
			wantSub: "handshake есть",
		},
		{
			name:    "stale handshake",
			hs:      AWGHandshake{Present: true, Age: 36 * time.Minute},
			wantOK:  false,
			wantSub: "устарел",
		},
		{
			name:    "no handshake",
			hs:      AWGHandshake{Present: true},
			wantOK:  false,
			wantSub: "нет handshake",
		},
		{
			name:    "missing iface",
			wantOK:  false,
			wantSub: "нет интерфейса AWG",
		},
	}
	for _, tc := range cases {
		ok, delay, detail := OverlayAWG2Health(tc.httpDelay, tc.hs)
		if ok != tc.wantOK || delay != tc.wantDelay {
			t.Fatalf("%s: ok=%v delay=%d want ok=%v delay=%d detail=%q",
				tc.name, ok, delay, tc.wantOK, tc.wantDelay, detail)
		}
		if tc.wantSub != "" && !strings.Contains(detail, tc.wantSub) {
			t.Fatalf("%s: detail=%q want substring %q", tc.name, detail, tc.wantSub)
		}
		if tc.wantSub == "" && detail != "" {
			t.Fatalf("%s: unexpected detail %q", tc.name, detail)
		}
	}
}
