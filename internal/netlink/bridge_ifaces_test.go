package netlink

import "testing"

func TestExpandSourceIfacesDedupes(t *testing.T) {
	got := expandSourceIfaces([]string{"br-lan", "br-lan", " phy0-ap0 "})
	if len(got) != 2 || got[0] != "br-lan" || got[1] != "phy0-ap0" {
		t.Fatalf("expandSourceIfaces() = %v, want [br-lan phy0-ap0]", got)
	}
}

func TestExpandSourceIfacesKeepsNonBridge(t *testing.T) {
	got := expandSourceIfaces([]string{"eth0"})
	if len(got) != 1 || got[0] != "eth0" {
		t.Fatalf("expandSourceIfaces() = %v, want [eth0]", got)
	}
}
