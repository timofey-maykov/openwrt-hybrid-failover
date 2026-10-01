package routing

import (
	"strings"
	"testing"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
)

func TestResolveChannel(t *testing.T) {
	sec := routesreport.Section{Channels: []routesreport.Channel{
		{ID: "vpn", Name: "VPN awg0"}, {ID: "a1b2c3d4", Name: "AWG Амстердам"}, {ID: "ffee0011", Name: "AWG Франкфурт"},
	}}
	cases := map[string]string{"2": "a1b2c3d4", "ffee0011": "ffee0011", "vpn": "vpn", "пул": "auto", "balance": "balance"}
	for in, want := range cases {
		got, err := ResolveChannel(sec, in)
		if err != nil || got != want {
			t.Fatalf("%q: got %q %v, want %q", in, got, err, want)
		}
	}
	if _, err := ResolveChannel(sec, "awg"); err == nil {
		t.Fatal("ambiguous prefix must fail")
	}
	if _, err := ResolveChannel(sec, "9"); err == nil {
		t.Fatal("out of range number must fail")
	}
}

func TestRouteSectionNameIsUCISafe(t *testing.T) {
	if got := RouteSectionName("main", "user:work-1"); got != "lr_main_user_work_1" {
		t.Fatal(got)
	}
}

func TestFormatRoutes(t *testing.T) {
	up := true
	out := FormatRoutes([]routesreport.Section{{
		Name:     "main",
		Channels: []routesreport.Channel{{ID: "a", Name: "AWG A", Up: &up, DelayMs: 90}},
		Lists:    []routesreport.List{{Key: "youtube", Kind: "community", Channel: "a"}, {Key: "news", Kind: "community", Channel: "auto"}},
		Bindings: []routesreport.Binding{{Name: "lr_main_youtube", Lists: []string{"youtube"}, Channel: "a", Via: "pool"}},
	}})
	for _, want := range []string{"1. ✅ AWG A 90 мс", "youtube → AWG A (канал упал, сейчас через пул)", "news → пул"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}
