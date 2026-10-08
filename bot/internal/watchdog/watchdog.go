// Package watchdog is the bot's outer safety net. Core already repairs what
// it can from inside (nft rules, default route, DNS failsafe, engine restart),
// but it cannot tell anybody, and it cannot notice that it is itself stuck.
// The watchdog checks each router from outside on a timer, restarts the
// service when a problem outlives core's own repair, and notifies the admins
// when that did not help and again when things are back.
package watchdog

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
)

// Router is what the watchdog needs from a managed router.
type Router interface {
	StatusReport(ctx context.Context) (diag.Report, error)
	Restart(ctx context.Context) error
	HardRestart(ctx context.Context) error
	Logs(ctx context.Context, lines int) (string, error)
}

// Target is one router under watch.
type Target struct {
	ID     string
	Name   string
	Router Router
}

type Config struct {
	// Interval between checks of every router.
	Interval time.Duration
	// FailThreshold is how many checks in a row must fail before anything is
	// done, so a reload that takes a few seconds is not an incident.
	FailThreshold int
	// AutoRepair lets the watchdog restart the service.
	AutoRepair bool
	// MaxRepairs per incident; after that the admins are told.
	MaxRepairs int
	// RepairWait is how long to let a repair take effect before judging it.
	RepairWait time.Duration
	// Renotify repeats a "still broken" notice this often.
	Renotify time.Duration
}

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = 30 * time.Second
	}
	if c.FailThreshold <= 0 {
		c.FailThreshold = 3
	}
	if c.MaxRepairs <= 0 {
		c.MaxRepairs = 2
	}
	if c.RepairWait <= 0 {
		c.RepairWait = 60 * time.Second
	}
	if c.Renotify <= 0 {
		c.Renotify = 30 * time.Minute
	}
	return c
}

// Problem is one failed check.
type Problem struct {
	Text string
	// Repairable problems are worth a restart. Losing every upstream channel
	// is not: restarting does not bring the provider back.
	Repairable bool
}

// Evaluate turns a status report (or the error from asking for it) into the
// list of problems. An empty list means healthy.
func Evaluate(rep diag.Report, err error) []Problem {
	if err != nil {
		return []Problem{{Text: "ядро не отвечает: " + oneLine(err.Error(), 160), Repairable: true}}
	}
	var out []Problem
	if !rep.EngineRunning && !rep.SingboxRunning {
		out = append(out, Problem{Text: "движок остановлен", Repairable: true})
	} else if !rep.NFTOK {
		out = append(out, Problem{Text: "правила nft не в порядке", Repairable: true})
	}
	if rep.EngineMode == "native" && rep.EngineRunning && !rep.FakeIPSkipped && rep.FakeIPOK != nil && !*rep.FakeIPOK {
		out = append(out, Problem{Text: "FakeIP DNS не отвечает", Repairable: true})
	}
	total, probed, up := 0, 0, 0
	for _, ch := range rep.Channels {
		if ch.Type == "urltest" {
			continue
		}
		total++
		if ch.Probed {
			probed++
		}
		if ch.Available {
			up++
		}
	}
	if total > 0 && probed == total && up == 0 {
		out = append(out, Problem{Text: fmt.Sprintf("все каналы недоступны (%d из %d не отвечают)", total, total)})
	}
	return out
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// State is a snapshot of one router for the UI.
type State struct {
	ID, Name   string
	Healthy    bool
	Checked    time.Time
	Problems   []string
	Since      time.Time // start of the open incident
	Repairs    int
	LastAction string
	// Last closed incident, if any.
	LastIncident *Incident
}

// Incident is a finished problem.
type Incident struct {
	From, To time.Time
	Problems []string
	Actions  []string
}

type target struct {
	Target
	fails        int
	problems     []Problem
	checked      time.Time
	open         bool
	since        time.Time
	repairs      int
	actions      []string
	lastRepair   time.Time
	lastNotify   time.Time
	spoke        bool // the admins heard about this incident
	problemTexts []string
	last         *Incident
	busy         bool
}

type Watchdog struct {
	cfg  Config
	log  *slog.Logger
	now  func() time.Time
	mu   sync.Mutex
	list []*target
	send func(string)

	mutedUntil time.Time
}

func New(targets []Target, cfg Config, log *slog.Logger) *Watchdog {
	w := &Watchdog{cfg: cfg.withDefaults(), log: log, now: time.Now}
	if w.log == nil {
		w.log = slog.Default()
	}
	for _, t := range targets {
		w.list = append(w.list, &target{Target: t})
	}
	return w
}

// SetNotifier sets where notices go (the admins' chats).
func (w *Watchdog) SetNotifier(send func(string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.send = send
}

func (w *Watchdog) Config() Config { return w.cfg }

// Mute silences notices for d, for planned work. Repairs continue.
func (w *Watchdog) Mute(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mutedUntil = w.now().Add(d)
}

func (w *Watchdog) Unmute() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.mutedUntil = time.Time{}
}

func (w *Watchdog) MutedUntil() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.now().Before(w.mutedUntil) {
		return w.mutedUntil
	}
	return time.Time{}
}

// notify sends text unless muted. Caller holds no lock.
func (w *Watchdog) notify(text string) {
	w.mu.Lock()
	send, muted := w.send, w.now().Before(w.mutedUntil)
	w.mu.Unlock()
	if muted || send == nil {
		w.log.Info("watchdog notice withheld", "muted", muted, "text", oneLine(text, 120))
		return
	}
	send(text)
}

// Run checks every router each interval until ctx ends.
func (w *Watchdog) Run(ctx context.Context) {
	t := time.NewTicker(w.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Tick(ctx)
		}
	}
}

// Tick runs one round over all routers.
func (w *Watchdog) Tick(ctx context.Context) {
	for _, t := range w.list {
		if ctx.Err() != nil {
			return
		}
		w.check(ctx, t)
	}
}

func (w *Watchdog) find(id string) *target {
	for _, t := range w.list {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// CheckNow checks one router right away and returns its state.
func (w *Watchdog) CheckNow(ctx context.Context, id string) (State, bool) {
	t := w.find(id)
	if t == nil {
		return State{}, false
	}
	w.check(ctx, t)
	return w.state(t), true
}

// RepairNow restarts one router's service on request and reports what happened.
func (w *Watchdog) RepairNow(ctx context.Context, id string) (string, error) {
	t := w.find(id)
	if t == nil {
		return "", fmt.Errorf("роутер %q не найден", id)
	}
	w.mu.Lock()
	if t.busy {
		w.mu.Unlock()
		return "", fmt.Errorf("ремонт уже идёт")
	}
	t.busy = true
	w.mu.Unlock()
	defer func() { w.mu.Lock(); t.busy = false; w.mu.Unlock() }()

	err := w.restart(ctx, t, 0)
	w.mu.Lock()
	t.lastRepair = w.now()
	t.actions = append(t.actions, "перезапуск по запросу")
	w.mu.Unlock()
	if err != nil {
		return "", err
	}
	return "Сервис перезапущен", nil
}

func (w *Watchdog) restart(ctx context.Context, t *target, step int) error {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	if step >= 1 {
		return t.Router.HardRestart(ctx)
	}
	return t.Router.Restart(ctx)
}

func (w *Watchdog) check(ctx context.Context, t *target) {
	cctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	rep, err := t.Router.StatusReport(cctx)
	cancel()
	problems := Evaluate(rep, err)

	w.mu.Lock()
	if t.busy {
		w.mu.Unlock()
		return
	}
	now := w.now()
	t.checked = now
	t.problems = problems
	t.problemTexts = t.problemTexts[:0]
	for _, p := range problems {
		t.problemTexts = append(t.problemTexts, p.Text)
	}

	if len(problems) == 0 {
		w.recoverLocked(t, now)
		return
	}
	t.fails++
	if t.fails < w.cfg.FailThreshold {
		w.mu.Unlock()
		return
	}
	if !t.open {
		t.open, t.since, t.repairs, t.actions, t.spoke = true, now, 0, nil, false
		t.lastNotify = time.Time{}
	}
	repairable := false
	for _, p := range problems {
		repairable = repairable || p.Repairable
	}
	canRepair := w.cfg.AutoRepair && repairable && t.repairs < w.cfg.MaxRepairs &&
		(t.lastRepair.IsZero() || now.Sub(t.lastRepair) >= w.cfg.RepairWait)
	name, texts := t.Name, strings.Join(t.problemTexts, "; ")
	step := t.repairs
	if canRepair {
		t.busy = true
		t.repairs++
		t.lastRepair = now
	}
	w.mu.Unlock()

	if canRepair {
		w.repair(ctx, t, name, texts, step)
		return
	}
	w.maybeGiveUp(ctx, t, name, texts, repairable)
}

func (w *Watchdog) repair(ctx context.Context, t *target, name, texts string, step int) {
	what := "перезапуск сервиса"
	if step >= 1 {
		what = "полная остановка и запуск сервиса"
	}
	w.notify(fmt.Sprintf("⚠ %s: %s.\nПробую починить: %s (%d из %d).", name, texts, what, step+1, w.cfg.MaxRepairs))
	w.mu.Lock()
	t.spoke = true
	w.mu.Unlock()

	err := w.restart(ctx, t, step)
	w.mu.Lock()
	result := what
	if err != nil {
		result += ": ошибка " + oneLine(err.Error(), 120)
	}
	t.actions = append(t.actions, result)
	t.busy = false
	t.fails = w.cfg.FailThreshold // judge the result on the next check
	w.mu.Unlock()
	w.log.Warn("watchdog repair", "router", t.ID, "step", step+1, "problems", texts, "err", err)
}

// maybeGiveUp tells the admins that the problem stays, once and then every
// Renotify. Nothing more can be done automatically at that point.
func (w *Watchdog) maybeGiveUp(ctx context.Context, t *target, name, texts string, repairable bool) {
	w.mu.Lock()
	now := w.now()
	due := t.lastNotify.IsZero() || now.Sub(t.lastNotify) >= w.cfg.Renotify
	exhausted := !repairable || !w.cfg.AutoRepair || t.repairs >= w.cfg.MaxRepairs
	waiting := repairable && !t.lastRepair.IsZero() && now.Sub(t.lastRepair) < w.cfg.RepairWait
	actions := append([]string(nil), t.actions...)
	since := t.since
	if !exhausted || waiting || !due {
		w.mu.Unlock()
		return
	}
	t.lastNotify, t.spoke = now, true
	w.mu.Unlock()

	var b strings.Builder
	switch {
	case !repairable:
		fmt.Fprintf(&b, "🚨 %s: %s.\nПерезапуск тут не поможет, смотрите провайдера и подписки.", name, texts)
	case !w.cfg.AutoRepair:
		fmt.Fprintf(&b, "🚨 %s: %s.\nАвтоматическая починка выключена.", name, texts)
	default:
		fmt.Fprintf(&b, "🚨 %s: не удалось починить за %d попытки.\nПроблема: %s.", name, len(actions), texts)
	}
	fmt.Fprintf(&b, "\nДлится: %s.", humanDuration(now.Sub(since)))
	for i, a := range actions {
		fmt.Fprintf(&b, "\n%d) %s", i+1, a)
	}
	if repairable {
		lctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		logs, err := t.Router.Logs(lctx, 12)
		cancel()
		if err == nil && strings.TrimSpace(logs) != "" {
			b.WriteString("\n\nПоследние строки лога:\n" + routing.MaskSecrets(trimLines(logs, 12)))
		}
	}
	w.notify(b.String())
}

func trimLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i, l := range lines {
		lines[i] = oneLine(l, 200)
	}
	return strings.Join(lines, "\n")
}

// recoverLocked closes an open incident. It is entered with w.mu held and
// leaves it released.
func (w *Watchdog) recoverLocked(t *target, now time.Time) {
	t.fails = 0
	if !t.open {
		w.mu.Unlock()
		return
	}
	inc := &Incident{From: t.since, To: now, Problems: append([]string(nil), t.problemTexts...), Actions: append([]string(nil), t.actions...)}
	spoke, name, actions := t.spoke, t.Name, append([]string(nil), t.actions...)
	t.last, t.open, t.repairs, t.actions, t.spoke = inc, false, 0, nil, false
	t.lastNotify = time.Time{}
	w.mu.Unlock()
	if !spoke {
		return
	}
	msg := fmt.Sprintf("✅ %s: всё восстановилось через %s.", name, humanDuration(now.Sub(inc.From)))
	if len(actions) > 0 {
		msg += "\nЧто делали: " + strings.Join(actions, ", ") + "."
	} else {
		msg += "\nСамо, без вмешательства."
	}
	w.notify(msg)
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d с", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d мин", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d ч %d мин", int(d.Hours()), int(d.Minutes())%60)
	}
}

func (w *Watchdog) state(t *target) State {
	w.mu.Lock()
	defer w.mu.Unlock()
	st := State{ID: t.ID, Name: t.Name, Healthy: len(t.problems) == 0, Checked: t.checked,
		Problems: append([]string(nil), t.problemTexts...), Repairs: t.repairs, LastIncident: t.last}
	if t.open {
		st.Since = t.since
	}
	if n := len(t.actions); n > 0 {
		st.LastAction = t.actions[n-1]
	}
	return st
}

// Snapshot is the state of every router, in a stable order.
func (w *Watchdog) Snapshot() []State {
	out := make([]State, 0, len(w.list))
	for _, t := range w.list {
		out = append(out, w.state(t))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
