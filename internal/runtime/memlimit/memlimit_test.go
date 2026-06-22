package memlimit

import "testing"

func TestParseLimit(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"64MiB", 64 << 20, true},
		{"80M", 80 << 20, true},
		{"48", 48, true},
		{"", 0, false},
		{"0M", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseLimit(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parseLimit(%q) = (%d, %v), want (%d, %v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
