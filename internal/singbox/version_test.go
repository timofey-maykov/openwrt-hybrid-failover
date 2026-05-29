package singbox

import "testing"

func TestCompareVersion(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.12.4", "1.12.4", 0},
		{"1.12.5", "1.12.4", 1},
		{"1.11.7", "1.12.4", -1},
		{"1.12.10", "1.12.4", 1},
	}
	for _, tc := range tests {
		got := compareVersion(tc.a, tc.b)
		if got != tc.want {
			t.Fatalf("compareVersion(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseSingboxVersion(t *testing.T) {
	got := parseSingboxVersion("sing-box version 1.12.4\nEnvironment: linux/amd64\n")
	if got != "1.12.4" {
		t.Fatalf("parseSingboxVersion = %q", got)
	}
}
