package netlink

import "testing"

func TestDockerBridgeName(t *testing.T) {
	for name, want := range map[string]bool{
		"docker0":          true,
		"br-337bebaac75d":  true,
		"br-lan":           false,
		"br-337bebaac75":   false,
		"br-337bebaac75dd": false,
		"br-ZZZZZZZZZZZZ":  false,
		"veth1234":         false,
	} {
		if got := dockerBridgeName.MatchString(name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"br-lan", "docker0"}, "docker0", "br-337bebaac75d")
	if len(got) != 3 || got[2] != "br-337bebaac75d" {
		t.Fatalf("appendUnique() = %v", got)
	}
}
