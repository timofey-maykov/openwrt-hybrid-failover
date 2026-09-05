package lifecycle

import "testing"

func TestShouldBounceStaleTunnel(t *testing.T) {
	if shouldBounceStaleTunnel("awg0") {
		t.Fatal("netifd primary must not be flapped by watchdog")
	}
	if shouldBounceStaleTunnel("wg0") {
		t.Fatal("netifd wireguard must not be flapped by watchdog")
	}
	if shouldBounceStaleTunnel("pawgc54d6d4c") {
		t.Fatal("pawg ifaces are recovered by SetupAWG2FromUCI, not bounce")
	}
}
