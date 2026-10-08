package outbound

import (
	"fmt"
	"testing"
	"time"
)

func TestSiteKey(t *testing.T) {
	cases := map[string]string{
		"www.youtube.com:443":                  "youtube.com",
		"i.ytimg.com":                          "ytimg.com",
		"rr3---sn-abc.googlevideo.com:443":     "rr3---sn-abc.googlevideo.com",
		"scontent-arn2-1.cdninstagram.com:443": "scontent-arn2-1.cdninstagram.com",
		"cdn4.telesco.pe:443":                  "cdn4.telesco.pe",
		"googlevideo.com":                      "googlevideo.com",
		"news.bbc.co.uk:443":                   "bbc.co.uk",
		"shop.example.com.au":                  "example.com.au",
		"t.me:443":                             "t.me",
		"149.154.167.51:443":                   "149.154.167.51",
		"WWW.Example.ORG.":                     "example.org",
	}
	for in, want := range cases {
		if got := siteKey(in); got != want {
			t.Errorf("siteKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassifyTier(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	cases := []struct {
		delay, best time.Duration
		prev        int
		hasPrev     bool
		want        int
	}{
		{ms(270), ms(270), 0, false, tierFast},
		{ms(390), ms(270), 0, false, tierFast},       // 1.44x
		{ms(60), ms(20), 0, false, tierFast},         // 3x but only 40 ms apart
		{ms(500), ms(270), 0, false, tierMedium},     // 1.85x
		{ms(800), ms(270), 0, false, tierSlow},       // 2.96x
		{0, ms(270), 0, false, tierFast},             // not probed yet
		{ms(420), ms(270), tierFast, true, tierFast}, // 1.56x, kept by hysteresis
		{ms(420), ms(270), tierMedium, true, tierMedium},
		{ms(700), ms(270), tierSlow, true, tierSlow},     // 2.59x, kept slow
		{ms(700), ms(270), tierMedium, true, tierMedium}, // 2.59x, kept medium
		{ms(540), ms(270), tierSlow, true, tierMedium},   // 2.0x, back to medium
	}
	for _, c := range cases {
		if got := classifyTier(c.delay, c.best, c.prev, c.hasPrev); got != c.want {
			t.Errorf("classifyTier(%v, %v, prev=%d/%v) = %d, want %d", c.delay, c.best, c.prev, c.hasPrev, got, c.want)
		}
	}
}

// The case found on a live router: five channels, one AWG at ~800 ms. Before,
// all YouTube video landed on that slow channel.
func routerMembers() []balanceMember {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	return []balanceMember{
		{tag: "glob-1-out", delay: ms(270)},
		{tag: "glob-2-out", delay: ms(385)},
		{tag: "glob-3-out", delay: ms(395)},
		{tag: "glob-4-out", delay: ms(390)},
		{tag: "glob-5-out", delay: ms(800)},
	}
}

func TestBalanceSkipsSlowChannel(t *testing.T) {
	b := &balanceHandler{}
	count := map[string]int{}
	for i := 0; i < 4000; i++ {
		order := b.rank(siteKey(fmt.Sprintf("rr%d---sn-x%d.googlevideo.com:443", i%20, i)), routerMembers())
		count[order[0]]++
		if order[len(order)-1] != "glob-5-out" {
			t.Fatalf("slow channel must rank last, got %v", order)
		}
	}
	if count["glob-5-out"] != 0 {
		t.Fatalf("slow channel picked %d times", count["glob-5-out"])
	}
	for _, tag := range []string{"glob-1-out", "glob-2-out", "glob-3-out", "glob-4-out"} {
		if share := float64(count[tag]) / 4000; share < 0.18 || share > 0.32 {
			t.Errorf("%s share %.2f, want about 0.25 (%v)", tag, share, count)
		}
	}
}

func TestBalanceMediumGetsHalfShare(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	members := []balanceMember{
		{tag: "a", delay: ms(200)},
		{tag: "b", delay: ms(200)},
		{tag: "c", delay: ms(400)}, // 2x: medium
	}
	b := &balanceHandler{}
	count := map[string]int{}
	for i := 0; i < 6000; i++ {
		count[b.rank(fmt.Sprintf("site%d.example", i), members)[0]]++
	}
	// Weights 1, 1, 0.5: c gets about a fifth.
	if share := float64(count["c"]) / 6000; share < 0.15 || share > 0.25 {
		t.Fatalf("medium share %.2f, want about 0.2 (%v)", share, count)
	}
}

func TestBalanceSiteIsSticky(t *testing.T) {
	b := &balanceHandler{}
	first := b.rank("youtube.com", routerMembers())[0]
	for i := 0; i < 50; i++ {
		if got := b.rank("youtube.com", routerMembers())[0]; got != first {
			t.Fatalf("site moved from %s to %s without any change", first, got)
		}
	}
}

func TestBalanceDownAndUnprobed(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	b := &balanceHandler{}
	members := []balanceMember{
		{tag: "a", down: true},
		{tag: "b", delay: ms(300)},
		{tag: "c"}, // no probe yet
	}
	for i := 0; i < 200; i++ {
		order := b.rank(fmt.Sprintf("s%d.example", i), members)
		if order[2] != "a" {
			t.Fatalf("down member must be last: %v", order)
		}
	}
	// Only slow members alive: they are still used, before down ones.
	members = []balanceMember{
		{tag: "a", down: true},
		{tag: "b", delay: ms(900)},
		{tag: "c", delay: ms(300)},
	}
	b = &balanceHandler{}
	order := b.rank("x.example", members)
	if order[0] != "c" || order[1] != "b" || order[2] != "a" {
		t.Fatalf("order %v, want c b a", order)
	}
}
