package outbound

import (
	"testing"
	"time"
)

func TestNextProbeWaitHealthyKeepsPeriod(t *testing.T) {
	if got := nextProbeWait(10*time.Minute, 0, 3*time.Second); got != 10*time.Minute-3*time.Second {
		t.Fatalf("got %v", got)
	}
	// a round that took longer than the interval does not spin
	if got := nextProbeWait(10*time.Second, 0, 20*time.Second); got != time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestNextProbeWaitBacksOffWhileNothingAnswers(t *testing.T) {
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, time.Minute, time.Minute, time.Minute}
	for i, w := range want {
		if got := nextProbeWait(10*time.Minute, i+1, 0); got != w {
			t.Fatalf("failed round %d: got %v, want %v", i+1, got, w)
		}
	}
}

func TestNextProbeWaitNeverLongerThanInterval(t *testing.T) {
	if got := nextProbeWait(8*time.Second, 3, 0); got != 8*time.Second {
		t.Fatalf("got %v", got)
	}
	if got := nextProbeWait(30*time.Second, 5, 0); got != 30*time.Second {
		t.Fatalf("got %v", got)
	}
}
