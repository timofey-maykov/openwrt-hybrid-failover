package lifecycle

import (
	"testing"
	"time"
)

func TestFastFailBackoffNeverZero(t *testing.T) {
	max := 30 * time.Second
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	for i, w := range want {
		if got := fastFailBackoff(i+1, max); got != w {
			t.Fatalf("failures=%d: got %s, want %s", i+1, got, w)
		}
	}
	// The raw shift overflowed past 63 failures and produced a zero delay,
	// which is what let a stuck bind restart thousands of times a minute.
	for _, n := range []int{63, 64, 100, 28647} {
		if got := fastFailBackoff(n, max); got != max {
			t.Fatalf("failures=%d: got %s, want %s", n, got, max)
		}
	}
}
