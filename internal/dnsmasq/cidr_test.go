package dnsmasq

import "testing"

func TestStripCIDR(t *testing.T) {
	cases := map[string]string{
		"192.168.1.1/24": "192.168.1.1",
		" 192.168.8.1 ":  "192.168.8.1",
		"10.0.0.1":       "10.0.0.1",
		"":               "",
	}
	for in, want := range cases {
		if got := stripCIDR(in); got != want {
			t.Errorf("stripCIDR(%q) = %q, want %q", in, got, want)
		}
	}
}
