package watchdog

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
)

func okReport() diag.Report {
	return diag.Report{
		EngineMode: "native", EngineRunning: true, NFTOK: true,
		Channels: []diag.ChannelStatus{
			{Name: "t", Type: "urltest", Available: true},
			{Name: "a", Type: "hy2", Available: true, Probed: true},
			{Name: "b", Type: "hy2", Probed: true},
		},
	}
}

type fakeRouter struct {
	mu        sync.Mutex
	rep       diag.Report
	statusErr error
	restarts  int
	hard      int
	restartFn func(*fakeRouter, string)
	logs      string
}

func (f *fakeRouter) StatusReport(context.Context) (diag.Report, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rep, f.statusErr
}

func (f *fakeRouter) Restart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.restarts++
	if f.restartFn != nil {
		f.restartFn(f, "restart")
	}
	return nil
}

func (f *fakeRouter) HardRestart(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hard++
	if f.restartFn != nil {
		f.restartFn(f, "hard")
	}
	return nil
}

func (f *fakeRouter) Logs(context.Context, int) (string, error) { return f.logs, nil }

func (f *fakeRouter) heal() {
	f.rep, f.statusErr = okReport(), nil
}

type rig struct {
	w      *Watchdog
	r      *fakeRouter
	clock  time.Time
	notes  []string
	notesM sync.Mutex
}

func newRig(t *testing.T, cfg Config) *rig {
	t.Helper()
	g := &rig{r: &fakeRouter{rep: okReport()}, clock: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)}
	g.w = New([]Target{{ID: "r1", Name: "OpenWrt", Router: g.r}}, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	g.w.now = func() time.Time { return g.clock }
	g.w.SetNotifier(func(s string) {
		g.notesM.Lock()
		g.notes = append(g.notes, s)
		g.notesM.Unlock()
	})
	return g
}

// tick advances the clock one interval and runs a round.
func (g *rig) tick() {
	g.clock = g.clock.Add(g.w.cfg.Interval)
	g.w.Tick(context.Background())
}

func (g *rig) ticks(n int) {
	for i := 0; i < n; i++ {
		g.tick()
	}
}

func (g *rig) said(want ...string) string {
	g.notesM.Lock()
	defer g.notesM.Unlock()
	all := strings.Join(g.notes, "\n---\n")
	for _, w := range want {
		if !strings.Contains(all, w) {
			panic("missing " + w + " in:\n" + all)
		}
	}
	return all
}

func (g *rig) count() int {
	g.notesM.Lock()
	defer g.notesM.Unlock()
	return len(g.notes)
}

func cfg() Config {
	return Config{Interval: 30 * time.Second, FailThreshold: 3, AutoRepair: true, MaxRepairs: 2, RepairWait: 60 * time.Second, Renotify: 30 * time.Minute}
}

func TestHealthyRouterIsSilent(t *testing.T) {
	g := newRig(t, cfg())
	g.ticks(20)
	if g.count() != 0 || g.r.restarts != 0 {
		t.Fatalf("a healthy router got attention: notes=%v restarts=%d", g.notes, g.r.restarts)
	}
	st := g.w.Snapshot()[0]
	if !st.Healthy || st.Checked.IsZero() {
		t.Fatalf("state: %+v", st)
	}
}

func TestShortBlipIsIgnored(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.EngineRunning = false
	g.ticks(2) // below the threshold of 3
	g.r.heal()
	g.ticks(5)
	if g.count() != 0 || g.r.restarts != 0 {
		t.Fatalf("reacted to a blip: notes=%v restarts=%d", g.notes, g.r.restarts)
	}
}

func TestSustainedFailureIsRestartedAndAnnounced(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.EngineRunning = false
	g.r.restartFn = func(f *fakeRouter, _ string) { f.heal() }
	g.ticks(3)
	if g.r.restarts != 1 || g.r.hard != 0 {
		t.Fatalf("restarts=%d hard=%d", g.r.restarts, g.r.hard)
	}
	g.said("⚠ OpenWrt: движок остановлен", "перезапуск сервиса (1 из 2)")
	g.tick()
	g.said("✅ OpenWrt: всё восстановилось", "перезапуск сервиса")
	if g.count() != 2 {
		t.Fatalf("want a start and a recovery notice, got %v", g.notes)
	}
	inc := g.w.Snapshot()[0].LastIncident
	if inc == nil || len(inc.Actions) != 1 {
		t.Fatalf("incident: %+v", inc)
	}
}

func TestSecondStepIsAHardRestart(t *testing.T) {
	g := newRig(t, cfg())
	g.r.statusErr = errors.New("dial unix: connection refused")
	g.r.restartFn = func(f *fakeRouter, kind string) {
		if kind == "hard" {
			f.heal()
		}
	}
	g.ticks(3) // threshold, first restart
	g.tick()   // 30s later: RepairWait (60s) not over, nothing new
	if g.r.restarts != 1 || g.r.hard != 0 {
		t.Fatalf("repaired too eagerly: restarts=%d hard=%d", g.r.restarts, g.r.hard)
	}
	g.tick() // 60s since the first repair
	if g.r.hard != 1 {
		t.Fatalf("hard restart not tried: restarts=%d hard=%d", g.r.restarts, g.r.hard)
	}
	g.said("полная остановка и запуск сервиса (2 из 2)")
	g.tick()
	g.said("✅ OpenWrt")
}

func TestGivesUpAndTellsTheAdminsWithLogs(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.NFTOK = false
	g.r.logs = "line1\nvless://uuid-1@host:443?token=SECRET#x\nline3"
	g.ticks(3)
	g.ticks(2)
	g.tick()
	g.ticks(2)
	g.tick() // second RepairWait over
	all := g.said("🚨 OpenWrt: не удалось починить", "правила nft не в порядке", "1) перезапуск сервиса", "2) полная остановка", "Последние строки лога", "line3")
	if strings.Contains(all, "SECRET") || strings.Contains(all, "uuid-1") {
		t.Fatalf("a credential reached the notice:\n%s", all)
	}
	n := g.count()
	g.ticks(10) // still broken, 5 minutes: no repeat before Renotify
	if g.count() != n || g.r.restarts != 1 || g.r.hard != 1 {
		t.Fatalf("kept pestering: notes=%d (was %d) restarts=%d hard=%d", g.count(), n, g.r.restarts, g.r.hard)
	}
	g.clock = g.clock.Add(31 * time.Minute)
	g.tick()
	if g.count() != n+1 {
		t.Fatalf("no reminder after %v", cfg().Renotify)
	}
	g.r.heal()
	g.tick()
	g.said("✅ OpenWrt: всё восстановилось")
}

func TestAllChannelsDownIsReportedNotRestarted(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.Channels = []diag.ChannelStatus{
		{Name: "t", Type: "urltest"},
		{Name: "a", Type: "hy2", Probed: true},
		{Name: "b", Type: "hy2", Probed: true},
	}
	g.ticks(3)
	g.said("🚨 OpenWrt: все каналы недоступны (2 из 2", "Перезапуск тут не поможет")
	if g.r.restarts != 0 || g.r.hard != 0 {
		t.Fatal("restarted for an upstream outage")
	}
	g.r.heal()
	g.tick()
	g.said("✅ OpenWrt", "Само, без вмешательства")
}

func TestUnprobedChannelsAreNotAnOutage(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.Channels = []diag.ChannelStatus{{Name: "a", Type: "hy2"}, {Name: "b", Type: "hy2"}}
	g.ticks(10)
	if g.count() != 0 {
		t.Fatalf("alarm on channels nobody probed yet: %v", g.notes)
	}
}

func TestAutoRepairOffOnlyTells(t *testing.T) {
	c := cfg()
	c.AutoRepair = false
	g := newRig(t, c)
	g.r.rep.EngineRunning = false
	g.ticks(3)
	g.said("🚨 OpenWrt: движок остановлен", "Автоматическая починка выключена")
	if g.r.restarts != 0 {
		t.Fatal("restarted although auto repair is off")
	}
}

func TestMuteWithholdsNoticesButNotRepairs(t *testing.T) {
	g := newRig(t, cfg())
	g.w.Mute(time.Hour)
	g.r.rep.EngineRunning = false
	g.r.restartFn = func(f *fakeRouter, _ string) { f.heal() }
	g.ticks(4)
	if g.count() != 0 {
		t.Fatalf("spoke while muted: %v", g.notes)
	}
	if g.r.restarts != 1 {
		t.Fatalf("muting must not stop repairs, restarts=%d", g.r.restarts)
	}
	if g.w.MutedUntil().IsZero() {
		t.Fatal("mute not reported")
	}
	g.w.Unmute()
	if !g.w.MutedUntil().IsZero() {
		t.Fatal("unmute ignored")
	}
}

func TestRepairNowRestartsOnRequest(t *testing.T) {
	g := newRig(t, cfg())
	msg, err := g.w.RepairNow(context.Background(), "r1")
	if err != nil || !strings.Contains(msg, "перезапущен") || g.r.restarts != 1 {
		t.Fatalf("msg=%q err=%v restarts=%d", msg, err, g.r.restarts)
	}
	if _, err := g.w.RepairNow(context.Background(), "nope"); err == nil {
		t.Fatal("unknown router accepted")
	}
}

func TestCheckNowReportsProblems(t *testing.T) {
	g := newRig(t, cfg())
	g.r.rep.NFTOK = false
	st, ok := g.w.CheckNow(context.Background(), "r1")
	if !ok || st.Healthy || len(st.Problems) != 1 || !strings.Contains(st.Problems[0], "nft") {
		t.Fatalf("state: %+v ok=%v", st, ok)
	}
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name string
		rep  diag.Report
		err  error
		want string
	}{
		{"ok", okReport(), nil, ""},
		{"rpc down", diag.Report{}, errors.New("exit 1"), "ядро не отвечает"},
		{"engine stopped", diag.Report{NFTOK: true}, nil, "движок остановлен"},
		{"legacy running", diag.Report{SingboxRunning: true, NFTOK: true}, nil, ""},
	}
	for _, c := range cases {
		ps := Evaluate(c.rep, c.err)
		got := ""
		if len(ps) > 0 {
			got = ps[0].Text
		}
		if c.want == "" && got != "" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
	rep := okReport()
	f := false
	rep.FakeIPOK = &f
	if ps := Evaluate(rep, nil); len(ps) != 1 || !ps[0].Repairable {
		t.Errorf("fakeip: %+v", ps)
	}
	rep.FakeIPSkipped = true
	if ps := Evaluate(rep, nil); len(ps) != 0 {
		t.Errorf("fakeip skipped must not alarm: %+v", ps)
	}
}
