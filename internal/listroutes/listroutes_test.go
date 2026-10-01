package listroutes

import "testing"

func TestSuggestSpreadsHeavyLists(t *testing.T) {
	got := Suggest([]string{"news", "youtube", "netflix", "telegram"}, []SuggestChannel{
		{ID: "slow", DelayMs: 300}, {ID: "fast", DelayMs: 80}, {ID: "dead", Down: true},
	})
	if got["youtube"] != "fast" || got["netflix"] != "slow" {
		t.Fatalf("heavy lists must split, fastest first: %v", got)
	}
	for _, ch := range got {
		if ch == "dead" {
			t.Fatalf("down channel used: %v", got)
		}
	}
	if len(Suggest([]string{"youtube"}, []SuggestChannel{{ID: "a"}})) != 0 {
		t.Fatal("one channel: nothing to suggest")
	}
}

func TestBoundListsFirstWins(t *testing.T) {
	m := BoundLists([]Route{
		{Name: "a", Lists: []string{"youtube"}, Channel: "x"},
		{Name: "b", Lists: []string{"youtube", "meta"}, Channel: "y"},
		{Name: "c", Lists: []string{"news"}, Channel: ChannelAuto},
	})
	if m["youtube"].Name != "a" || m["meta"].Name != "b" {
		t.Fatalf("%v", m)
	}
	if _, ok := m["news"]; ok {
		t.Fatal("auto binding takes nothing")
	}
}

func TestNormalizeCIDR(t *testing.T) {
	for in, want := range map[string]string{"10.0.0.5": "10.0.0.5/32", "10.20.1.0/16": "10.20.0.0/16"} {
		if got, ok := NormalizeCIDR(in); !ok || got != want {
			t.Fatalf("%s: %s %v", in, got, ok)
		}
	}
	for _, bad := range []string{"jira.example.com", "300.1.1.1", "::1", "10.0.0.0/40"} {
		if _, ok := NormalizeCIDR(bad); ok {
			t.Fatalf("%s accepted", bad)
		}
	}
}
