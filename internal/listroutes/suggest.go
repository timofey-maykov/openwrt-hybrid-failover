package listroutes

import "sort"

// serviceWeight is a rough share of traffic a service list brings: video
// streams dominate, chats and small sites barely count. Unknown lists weigh 2.
var serviceWeight = map[string]int{
	"youtube": 10, "netflix": 9, "hdrezka": 8, "anime": 7, "tiktok": 7, "twitch": 7,
	"meta": 5, "twitter": 4, "discord": 4, "google_play": 4, "roblox": 3, "rocketleague": 3,
	"telegram": 3, "google_ai": 2, "news": 1, "porn": 6,
}

// Weight returns the traffic weight of a list key.
func Weight(key string) int {
	if w, ok := serviceWeight[key]; ok {
		return w
	}
	return 2
}

// SuggestChannel is what Suggest needs to know about a channel.
type SuggestChannel struct {
	ID      string
	DelayMs int // >0 measured, <=0 unknown or down
	Down    bool
}

// Suggest spreads lists over live channels so heavy lists land on different
// channels: heaviest list first, each to the channel with the least weight so
// far; ties go to the faster channel. Returns list key → channel id.
func Suggest(lists []string, chans []SuggestChannel) map[string]string {
	var live []SuggestChannel
	for _, c := range chans {
		if !c.Down {
			live = append(live, c)
		}
	}
	out := make(map[string]string, len(lists))
	if len(live) < 2 {
		return out // nothing to spread over
	}
	sort.SliceStable(live, func(i, j int) bool { return delayRank(live[i]) < delayRank(live[j]) })
	sorted := append([]string(nil), lists...)
	sort.SliceStable(sorted, func(i, j int) bool { return Weight(sorted[i]) > Weight(sorted[j]) })
	load := make([]int, len(live))
	for _, l := range sorted {
		best := 0
		for i := range live {
			if load[i] < load[best] {
				best = i
			}
		}
		load[best] += Weight(l)
		out[l] = live[best].ID
	}
	return out
}

func delayRank(c SuggestChannel) int {
	if c.DelayMs > 0 {
		return c.DelayMs
	}
	return 1 << 30
}
