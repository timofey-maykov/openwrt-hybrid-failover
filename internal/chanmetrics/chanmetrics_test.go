package chanmetrics

import (
	"testing"
	"time"
)

func reading(key string, rx, tx, conns uint64, active int64, delay int) Reading {
	return Reading{Channel: Channel{Key: key}, RxBytes: rx, TxBytes: tx, Conns: conns, Active: active, DelayMs: delay}
}

func TestSamplerRatesAndReset(t *testing.T) {
	s := NewSampler()
	t0 := time.Unix(1_800_000_000, 0)
	s.Add(t0, []Reading{reading("m/a", 1000, 500, 1, 1, 80)})
	s.Add(t0.Add(2*time.Second), []Reading{reading("m/a", 5000, 2500, 3, 2, 80)})
	// counter reset (engine restart): the new value is the delta
	s.Add(t0.Add(4*time.Second), []Reading{reading("m/a", 400, 200, 1, 1, -1), reading("m/b", 10, 10, 0, 0, 0)})
	f := s.Fine()
	if len(f.T) != 2 {
		t.Fatalf("points %d, want 2 (first round only primes)", len(f.T))
	}
	a := f.Series["m/a"]
	if a.Rx[0] != 2000 || a.Tx[0] != 1000 || a.New[0] != 100 || a.Active[0] != 2 {
		t.Fatalf("first point %+v", a)
	}
	if a.Rx[1] != 200 || a.Delay[1] != -1 {
		t.Fatalf("after reset %+v", a)
	}
	b := f.Series["m/b"]
	if len(b.Rx) != 2 || b.Rx[0] != 0 {
		t.Fatalf("late channel must be padded: %+v", b)
	}
}

func TestSamplerMinuteRing(t *testing.T) {
	s := NewSampler()
	t0 := time.Unix(1_800_000_000, 0).Truncate(time.Minute)
	var rx uint64
	closed := 0
	for i := 0; i <= 31; i++ {
		rx += 2000
		if s.Add(t0.Add(time.Duration(i)*2*time.Second), []Reading{reading("m/a", rx, 0, 0, int64(i%5), 50)}) {
			closed++
		}
	}
	if closed != 1 {
		t.Fatalf("closed minutes %d", closed)
	}
	c := s.Coarse()
	if len(c.T) != 1 || c.Series["m/a"].Rx[0] != 1000 || c.Series["m/a"].Active[0] != 4 || c.Series["m/a"].Delay[0] != 50 {
		t.Fatalf("coarse %+v %+v", c.T, c.Series["m/a"])
	}
}

func TestRingTrim(t *testing.T) {
	r := newRing(3)
	for i := int64(1); i <= 5; i++ {
		r.push(i, map[string][5]int64{"k": {i, 0, 0, 0, 0}})
	}
	if len(r.t) != 3 || r.t[0] != 3 || r.series["k"].Rx[0] != 3 {
		t.Fatalf("trim %v %v", r.t, r.series["k"].Rx)
	}
}
