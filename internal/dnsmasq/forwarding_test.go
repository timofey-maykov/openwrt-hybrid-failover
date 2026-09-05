package dnsmasq

import "testing"

func TestLANForwardingOK(t *testing.T) {
	cases := []struct {
		server       string
		notinterface string
		want         bool
	}{
		{"127.0.0.42", "lo", true},
		{"127.0.0.42\n", "lo\n", true},
		{"127.0.0.42", "", false},
		{"127.0.0.42", "br-lan", false},
		{"1.1.1.1", "lo", false},
		{"", "lo", false},
	}
	for _, tc := range cases {
		got := lanForwardingOK(tc.server, tc.notinterface)
		if got != tc.want {
			t.Fatalf("server=%q notinterface=%q got %v want %v", tc.server, tc.notinterface, got, tc.want)
		}
	}
}

func TestShouldStopForFakeIP(t *testing.T) {
	cases := []struct {
		name         string
		lanOK        bool
		engineReady  bool
		fakeIPBusy   bool
		want         bool
	}{
		{name: "engine already on FakeIP, do not kill LAN DNS", lanOK: true, engineReady: true, fakeIPBusy: true, want: false},
		{name: "engine on FakeIP even if UCI not yet notinterface=lo", lanOK: false, engineReady: true, fakeIPBusy: true, want: false},
		{name: "leftover occupant, engine not ready", lanOK: true, engineReady: false, fakeIPBusy: true, want: true},
		{name: "UCI not forwarding, engine not ready", lanOK: false, engineReady: false, fakeIPBusy: false, want: true},
		{name: "already split and port free", lanOK: true, engineReady: false, fakeIPBusy: false, want: false},
	}
	for _, tc := range cases {
		got := ShouldStopForFakeIP(tc.lanOK, tc.engineReady, tc.fakeIPBusy)
		if got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}
