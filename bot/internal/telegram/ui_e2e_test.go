package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/audit"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/security"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/watchdog"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
)

const (
	adminID  = int64(1)
	viewerID = int64(2)
	chatID   = int64(100)
	subURL   = "https://sub.example.com/api/v1/s/SECRETTOKEN1234"
)

// fakeRouter emulates just enough of uci and core RPC for the bot screens.
type fakeRouter struct {
	mu       sync.Mutex
	subs     []string
	interval string
	opts     map[string]string
	staged   int
	down     bool // the engine is stopped until the service is restarted
	runs     []string
	rpc      [][]string
}

var uciWriteRe = regexp.MustCompile(`uci (add_list|del_list|set) (\S+?)='(.*)'$`)

func (f *fakeRouter) Run(ctx context.Context, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cmd := name + " " + strings.Join(args, " ")
	f.runs = append(f.runs, cmd)
	const subKey = "hybrid-failover.settings.subscription_urls"
	const ivKey = "hybrid-failover.settings.subscription_update_interval"
	if m := uciWriteRe.FindStringSubmatch(cmd); m != nil {
		op, key, val := m[1], m[2], m[3]
		f.staged++
		switch {
		case key == subKey && op == "add_list":
			f.subs = append(f.subs, val)
		case key == subKey && op == "del_list":
			for i, s := range f.subs {
				if s == val {
					f.subs = append(f.subs[:i], f.subs[i+1:]...)
					break
				}
			}
		case key == ivKey:
			f.interval = val
		default:
			if f.opts == nil {
				f.opts = map[string]string{}
			}
			f.opts[key] = val
		}
		return "", nil
	}
	if strings.HasSuffix(cmd, "/etc/init.d/hybrid-failover restart") || strings.HasSuffix(cmd, "/etc/init.d/hybrid-failover start") {
		f.down = false
	}
	switch {
	case strings.HasPrefix(cmd, "/sbin/uci -q get "):
		key := args[len(args)-1]
		switch key {
		case subKey:
			if len(f.subs) == 0 {
				return "", errors.New("exit status 1")
			}
			return strings.Join(f.subs, " "), nil
		case ivKey:
			if f.interval == "" {
				return "", errors.New("exit status 1")
			}
			return f.interval, nil
		}
		return "", errors.New("exit status 1")
	case strings.HasPrefix(cmd, "/sbin/uci get "):
		if v, ok := f.opts[args[len(args)-1]]; ok {
			return v, nil
		}
		return "", errors.New("exit status 1")
	case strings.HasPrefix(cmd, "/sbin/uci show"):
		return "hybrid-failover.settings.subscription_urls='" + strings.Join(f.subs, "' '") + "'", nil
	case strings.HasPrefix(cmd, "/sbin/uci changes"):
		return strings.Repeat("hybrid-failover.x=y\n", f.staged), nil
	}
	return "", nil
}

func (f *fakeRouter) RunBytes(ctx context.Context, name string, args ...string) ([]byte, error) {
	out, err := f.Run(ctx, name, args...)
	return []byte(out), err
}

func (f *fakeRouter) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rpc = append(f.rpc, append([]string{method}, args...))
	switch method {
	case "Status", "Health":
		return js(diag.Report{
			EngineMode: "native", EngineRunning: !f.down, NFTOK: true, ActiveOutbound: "glob-urltest-out",
			Failover: &diag.FailoverInfo{Section: "glob", Policy: "fastest", URLTestNow: "glob-1-out"},
			Channels: []diag.ChannelStatus{
				{Name: "glob-urltest-out", Display: "URLTest", Type: "urltest", Available: true, DelayMs: 272, Selected: true, Probed: true},
				{Name: "glob-1-out", Display: "77.110.127.13:443 (glob-1-out)", Type: "hy2", Available: true, DelayMs: 404, Probed: true},
				{Name: "glob-2-out", Display: "185.173.147.72:443 (glob-2-out)", Type: "hy2", Probed: true, Detail: "timeout"},
			},
		}), nil
	case "ListRoutes":
		up := true
		return js(routesreport.Report{Sections: []routesreport.Section{{
			Name:     "glob",
			Channels: []routesreport.Channel{{ID: "c1", Name: "AWG Amsterdam", Up: &up}, {ID: "c2", Name: "AWG Frankfurt", Up: &up}},
			Lists: []routesreport.List{
				{Key: "youtube", Kind: "community", Channel: "c1"},
				{Key: "news", Kind: "community", Channel: "auto"},
			},
		}}}), nil
	case "PendingApply":
		f.staged = 0
	}
	return "", nil
}

func js(v any) string { b, _ := json.Marshal(v); return string(b) }

type tgCall struct {
	method string
	form   url.Values
}

type fakeTelegram struct {
	t     *testing.T
	mu    sync.Mutex
	calls []tgCall
	srv   *httptest.Server
}

func newFakeTelegram(t *testing.T) *fakeTelegram {
	ft := &fakeTelegram{t: t}
	ft.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		ft.mu.Lock()
		ft.calls = append(ft.calls, tgCall{method: method, form: form})
		n := len(ft.calls)
		ft.mu.Unlock()
		switch method {
		case "getMe":
			_, _ = fmt.Fprint(w, `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"b","username":"b"}}`)
		case "sendMessage", "editMessageText":
			_, _ = fmt.Fprintf(w, `{"ok":true,"result":{"message_id":%d,"date":1,"chat":{"id":%d},"text":"x"}}`, n, chatID)
		default:
			_, _ = fmt.Fprint(w, `{"ok":true,"result":true}`)
		}
	}))
	t.Cleanup(ft.srv.Close)
	return ft
}

// last returns the newest message the bot sent or edited.
func (ft *fakeTelegram) last() tgCall {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	for i := len(ft.calls) - 1; i >= 0; i-- {
		if m := ft.calls[i].method; m == "sendMessage" || m == "editMessageText" {
			return ft.calls[i]
		}
	}
	ft.t.Fatal("bot sent nothing")
	return tgCall{}
}

func (ft *fakeTelegram) count(method string) int {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	n := 0
	for _, c := range ft.calls {
		if c.method == method {
			n++
		}
	}
	return n
}

// everything is every byte the bot ever sent, for leak checks.
func (ft *fakeTelegram) everything() string {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	var b strings.Builder
	for _, c := range ft.calls {
		b.WriteString(c.form.Encode())
		for _, v := range c.form {
			b.WriteString(strings.Join(v, ""))
		}
	}
	return b.String()
}

func (c tgCall) text() string { return c.form.Get("text") }

// buttons maps button label to callback data.
func (c tgCall) buttons() map[string]string {
	out := map[string]string{}
	var kb struct {
		Rows [][]struct {
			Text string `json:"text"`
			Data string `json:"callback_data"`
		} `json:"inline_keyboard"`
	}
	if raw := c.form.Get("reply_markup"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &kb)
	}
	for _, r := range kb.Rows {
		for _, b := range r {
			out[b.Text] = b.Data
		}
	}
	return out
}

func (c tgCall) hasData(data string) bool {
	for _, d := range c.buttons() {
		if d == data {
			return true
		}
	}
	return false
}

type harness struct {
	t      *testing.T
	bot    *Bot
	tg     *fakeTelegram
	router *fakeRouter
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	tg := newFakeTelegram(t)
	api, err := tgbotapi.NewBotAPIWithAPIEndpoint("TOKEN", tg.srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRouter{}
	svc := routing.NewService(fr, "http://127.0.0.1:1", "/etc/init.d/hybrid-failover", "", "glob", 5*time.Second)
	mgr := routers.NewStatic(routers.Instance{ID: "local", Name: "OpenWrt", Service: svc})
	h := NewCommandHandler(mgr, botconfig.Store{})
	b := New(api, security.NewAuthorizer([]int64{adminID}, []int64{viewerID}),
		audit.New(filepath.Join(t.TempDir(), "audit.log")), h, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &harness{t: t, bot: b, tg: tg, router: fr}
}

func (h *harness) say(user int64, text string) {
	h.t.Helper()
	h.bot.dispatchUpdate(context.Background(), tgbotapi.Update{Message: &tgbotapi.Message{
		Text: text, From: &tgbotapi.User{ID: user}, Chat: &tgbotapi.Chat{ID: chatID},
	}})
}

func (h *harness) press(user int64, data string) {
	h.t.Helper()
	h.bot.dispatchUpdate(context.Background(), tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		ID: "cb", From: &tgbotapi.User{ID: user}, Data: data,
		Message: &tgbotapi.Message{MessageID: 7, Chat: &tgbotapi.Chat{ID: chatID}},
	}})
}

func needs(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Fatalf("missing %q in:\n%s", w, got)
		}
	}
}

func TestMenuShowsLiveStatusAndSections(t *testing.T) {
	h := newHarness(t)
	h.say(adminID, "/menu")
	c := h.tg.last()
	needs(t, c.text(), "Hybrid Failover · OpenWrt", "Движок работает", "Активный: URLTest · 272 мс", "через 77.110.127.13:443", "Каналов доступно: 1 из 2")
	for _, data := range []string{"nav:channels", "nav:subs", "nav:lists", "nav:settings", "nav:system", "cmd:/health"} {
		if !c.hasData(data) {
			t.Fatalf("menu lacks %s: %v", data, c.buttons())
		}
	}
}

func TestEveryButtonFitsTelegramLimit(t *testing.T) {
	h := newHarness(t)
	h.router.subs = []string{subURL, subURL + "x"}
	for _, nav := range []string{"main", "channels", "subs", "lists", "lst:0", "settings", "system"} {
		h.press(adminID, "nav:"+nav)
		for label, data := range h.tg.last().buttons() {
			if len(data) > maxCallbackData {
				t.Fatalf("%s: button %q has %d byte payload", nav, label, len(data))
			}
		}
	}
}

func TestChannelsScreenSwitchesToARealChannel(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "nav:channels")
	c := h.tg.last()
	needs(t, c.text(), "▶ 🤖 URLTest", "77.110.127.13:443 (hy2) · 404 мс", "185.173.147.72:443 (hy2) · timeout")
	if !c.hasData("cmd:/switch glob-1-out") || !c.hasData("cmd:/switch glob-urltest-out") {
		t.Fatalf("channel buttons: %v", c.buttons())
	}
	if strings.Contains(c.text(), "Primary VPN") {
		t.Fatal("hardcoded channel names are back")
	}

	h.press(adminID, "cmd:/switch glob-1-out")
	var switched bool
	for _, r := range h.router.rpc {
		if r[0] == "SwitchProxy" && len(r) == 3 && r[1] == "glob" && r[2] == "glob-1-out" {
			switched = true
		}
	}
	if !switched {
		t.Fatalf("SwitchProxy not called: %v", h.router.rpc)
	}
	needs(t, h.tg.last().text(), "✔ Канал переключён", "📡 Каналы")
}

func TestSubscriptionLifecycle(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "nav:subs")
	needs(t, h.tg.last().text(), "Подписок нет")

	h.press(adminID, "input:sub_add")
	needs(t, h.tg.last().text(), "Пришлите ссылку")
	h.say(adminID, subURL)
	c := h.tg.last()
	needs(t, c.text(), "Подписка добавлена", "Подписки (1)", "https://sub.example.com/…1234", "Изменений в ожидании")
	if len(h.router.subs) != 1 || h.router.subs[0] != subURL {
		t.Fatalf("subs stored: %v", h.router.subs)
	}
	if !c.hasData("cmd:/param_apply") || !c.hasData("cmd:/sub_del 1") || !c.hasData("cmd:/sub_interval 6h") {
		t.Fatalf("buttons: %v", c.buttons())
	}

	h.press(adminID, "cmd:/sub_interval 6h")
	if h.router.interval != "6h" {
		t.Fatalf("interval: %q", h.router.interval)
	}
	needs(t, h.tg.last().text(), "каждые 6 ч")

	h.press(adminID, "cmd:/sub_del 1")
	needs(t, h.tg.last().text(), "Удалить подписку?", "https://sub.example.com/…1234")
	if len(h.router.subs) != 1 {
		t.Fatal("deleted before confirmation")
	}
	h.press(adminID, "confirm:/sub_del 1")
	if len(h.router.subs) != 0 {
		t.Fatalf("not deleted: %v", h.router.subs)
	}
	needs(t, h.tg.last().text(), "Подписка удалена", "Подписок нет")

	if strings.Contains(h.tg.everything(), "SECRETTOKEN") {
		t.Fatal("subscription token leaked into a Telegram message")
	}
}

func TestSubscriptionRejectsBadInput(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "input:sub_add")
	h.say(adminID, "not a url")
	needs(t, h.tg.last().text(), "Ошибка ввода")
	h.say(adminID, "ftp://host/x")
	needs(t, h.tg.last().text(), "нужна ссылка вида https")
	if len(h.router.subs) != 0 {
		t.Fatalf("stored garbage: %v", h.router.subs)
	}
	h.say(adminID, "/cancel")
	h.press(adminID, "input:sub_add")
	h.say(adminID, subURL)
	h.press(adminID, "input:sub_add")
	h.say(adminID, subURL)
	needs(t, h.tg.last().text(), "уже есть")
}

func TestSecretsAreMaskedInRawCommands(t *testing.T) {
	h := newHarness(t)
	h.router.subs = []string{subURL}
	h.say(adminID, "/uci_show")
	c := h.tg.last()
	needs(t, c.text(), "subscription_urls", "https://sub.example.com/…1234")
	if strings.Contains(c.text(), "SECRETTOKEN") {
		t.Fatalf("raw uci output echoed a secret: %s", c.text())
	}
	h.say(adminID, "/sub_del 1")
	if strings.Contains(h.tg.everything(), "SECRETTOKEN") {
		t.Fatal("a secret reached Telegram")
	}
}

func TestViewerSeesButCannotChange(t *testing.T) {
	h := newHarness(t)
	h.router.subs = []string{subURL}
	h.press(viewerID, "nav:subs")
	c := h.tg.last()
	needs(t, c.text(), "Подписки (1)")
	for label, data := range c.buttons() {
		if strings.HasPrefix(data, "cmd:/sub_") || strings.HasPrefix(data, "input:") || data == "cmd:/param_apply" {
			t.Fatalf("viewer got write button %q (%s)", label, data)
		}
	}
	before := h.tg.count("sendMessage") + h.tg.count("editMessageText")
	h.press(viewerID, "cmd:/sub_del 1")
	if len(h.router.subs) != 1 {
		t.Fatal("viewer deleted a subscription")
	}
	if h.tg.count("sendMessage")+h.tg.count("editMessageText") != before {
		t.Fatal("viewer got a confirmation prompt for a write action")
	}
	h.say(viewerID, "/sub_add "+subURL+"2")
	needs(t, h.tg.last().text(), "Доступ запрещен")
}

func TestUnknownUserGetsNothing(t *testing.T) {
	h := newHarness(t)
	h.say(999, "/menu")
	needs(t, h.tg.last().text(), "Доступ запрещен")
	h.press(999, "cmd:/switch glob-1-out")
	for _, r := range h.router.rpc {
		if r[0] == "SwitchProxy" {
			t.Fatal("stranger switched a channel")
		}
	}
}

func TestListsBindToChannel(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "nav:lists")
	c := h.tg.last()
	needs(t, c.text(), "youtube → AWG Amsterdam", "news → пул")
	if !c.hasData("nav:lst:0") {
		t.Fatalf("buttons: %v", c.buttons())
	}
	h.press(adminID, "nav:lst:0")
	c = h.tg.last()
	needs(t, c.text(), "youtube", "Сейчас: AWG Amsterdam")
	if !c.hasData("cmd:/rt 0 2") || !c.hasData("cmd:/rt 0 block") {
		t.Fatalf("buttons: %v", c.buttons())
	}
	h.press(adminID, "cmd:/rt 0 2")
	var bound bool
	for _, r := range h.router.runs {
		if strings.Contains(r, "lr_glob_youtube.channel='c2'") {
			bound = true
		}
	}
	if !bound {
		t.Fatalf("binding not written: %v", h.router.runs)
	}
	needs(t, h.tg.last().text(), "✔ Сохранено", "🗂 Списки")
}

func TestStaleListIndexIsRejected(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "cmd:/rt 9 1")
	needs(t, h.tg.last().text(), "Ошибка", "список не найден")
}

func TestSettingsQUICButtonDoesWhatItSays(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "nav:settings")
	c := h.tg.last()
	needs(t, c.text(), "QUIC: включён")
	if !c.hasData("cmd:/set_quic off") {
		t.Fatalf("buttons: %v", c.buttons())
	}
	h.press(adminID, "cmd:/set_quic off")
	if got := h.router.opts["hybrid-failover.settings.disable_quic"]; got != "1" {
		t.Fatalf("turning QUIC off must set disable_quic=1, got %q", got)
	}
	needs(t, h.tg.last().text(), "QUIC: выключен")
}

func TestApplyAsksForConfirmationThenApplies(t *testing.T) {
	h := newHarness(t)
	h.router.staged = 2
	h.press(adminID, "nav:main")
	c := h.tg.last()
	needs(t, c.text(), "Изменений в ожидании: 2")
	if !c.hasData("cmd:/param_apply") {
		t.Fatalf("buttons: %v", c.buttons())
	}
	h.press(adminID, "cmd:/param_apply")
	needs(t, h.tg.last().text(), "Применить все ожидающие изменения")
	h.press(adminID, "confirm:/param_apply")
	applied := false
	for _, r := range h.router.rpc {
		if r[0] == "PendingApply" {
			applied = true
		}
	}
	if !applied {
		t.Fatalf("apply never reached core: %v", h.router.rpc)
	}
	needs(t, h.tg.last().text(), "✔ Изменения применены", "🛰 Hybrid Failover")
	if strings.Contains(h.tg.last().text(), "Изменений в ожидании") {
		t.Fatal("pending banner stayed after apply")
	}
}

func TestStaleConfirmationDoesNothing(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "confirm:/param_apply")
	for _, r := range h.router.rpc {
		if r[0] == "PendingApply" {
			t.Fatal("applied without a fresh confirmation")
		}
	}
}

func TestSlashMenuRegistered(t *testing.T) {
	h := newHarness(t)
	h.bot.registerCommands()
	if h.tg.count("setMyCommands") != 1 {
		t.Fatal("slash menu not registered")
	}
}

func TestCommandShortcutsOpenScreens(t *testing.T) {
	h := newHarness(t)
	for cmd, want := range map[string]string{
		"/subs": "📰 Подписки", "/lists": "🗂 Списки", "/settings": "⚙️ Настройки",
		"/system": "🛠 Система", "/channels": "📡 Каналы", "/start": "🛰 Hybrid Failover",
	} {
		h.say(adminID, cmd)
		needs(t, h.tg.last().text(), want)
	}
}

// withWatchdog attaches a watchdog that reports to the admin chat.
func (h *harness) withWatchdog(cfg watchdog.Config) *watchdog.Watchdog {
	h.t.Helper()
	inst, err := h.bot.h.(CommandHandler).mgr.InstanceFor(adminID)
	if err != nil {
		h.t.Fatal(err)
	}
	wd := watchdog.New([]watchdog.Target{{ID: inst.ID, Name: inst.Name, Router: inst.Service}}, cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.bot.h = h.bot.h.(CommandHandler).WithWatchdog(wd)
	h.bot.SetIdentity("OpenWrt", []int64{adminID})
	wd.SetNotifier(h.bot.NotifyAdmins)
	return wd
}

func wdConfig() watchdog.Config {
	return watchdog.Config{Interval: 30 * time.Second, FailThreshold: 2, AutoRepair: true, MaxRepairs: 2, RepairWait: time.Nanosecond}
}

func (h *harness) restarts() int {
	n := 0
	for _, r := range h.router.runs {
		if strings.HasSuffix(r, "/etc/init.d/hybrid-failover restart") {
			n++
		}
	}
	return n
}

func TestWatchScreenAndMainSummary(t *testing.T) {
	h := newHarness(t)
	wd := h.withWatchdog(wdConfig())
	wd.Tick(context.Background())

	h.press(adminID, "nav:main")
	c := h.tg.last()
	needs(t, c.text(), "🛡 Мониторинг: всё в порядке")
	if !c.hasData("nav:watch") {
		t.Fatalf("no monitoring button: %v", c.buttons())
	}
	h.press(adminID, "nav:watch")
	c = h.tg.last()
	needs(t, c.text(), "Проверка каждые 30 с · авточинка: вкл", "🔔 Уведомления включены", "✅ OpenWrt: всё в порядке")
	for _, d := range []string{"nav:watch_check", "cmd:/repair", "cmd:/mute 60"} {
		if !c.hasData(d) {
			t.Fatalf("missing %s: %v", d, c.buttons())
		}
	}
}

func TestWatchdogFixesADeadEngineAndTellsTheAdmin(t *testing.T) {
	h := newHarness(t)
	wd := h.withWatchdog(wdConfig())
	h.router.down = true

	wd.Tick(context.Background()) // 1st failure: below the threshold
	if h.tg.count("sendMessage") != 0 || h.restarts() != 0 {
		t.Fatal("reacted to the first failed check")
	}
	wd.Tick(context.Background()) // 2nd: repair
	if h.restarts() != 1 {
		t.Fatalf("service not restarted: %v", h.router.runs)
	}
	needs(t, h.tg.last().text(), "⚠ OpenWrt: движок остановлен", "перезапуск сервиса (1 из 2)")
	if h.tg.last().form.Get("chat_id") != fmt.Sprint(adminID) {
		t.Fatal("notice did not go to the admin chat")
	}

	wd.Tick(context.Background()) // engine is back
	needs(t, h.tg.last().text(), "✅ OpenWrt: всё восстановилось", "перезапуск сервиса")
}

func TestWatchdogToldWhenItCannotFix(t *testing.T) {
	h := newHarness(t)
	wd := h.withWatchdog(wdConfig())
	h.router.down = true
	// A restart that does not help: the engine is down again at every check.
	for i := 0; i < 8; i++ {
		h.router.mu.Lock()
		h.router.down = true
		h.router.mu.Unlock()
		wd.Tick(context.Background())
	}
	needs(t, h.tg.last().text(), "🚨 OpenWrt: не удалось починить", "движок остановлен")
	h.press(adminID, "nav:watch")
	needs(t, h.tg.last().text(), "⚠ OpenWrt: проблема", "• движок остановлен")
}

func TestRepairNeedsConfirmation(t *testing.T) {
	h := newHarness(t)
	h.withWatchdog(wdConfig())
	h.press(adminID, "cmd:/repair")
	needs(t, h.tg.last().text(), "Перезапустить сервис?")
	if h.restarts() != 0 {
		t.Fatal("restarted before confirmation")
	}
	h.press(adminID, "confirm:/repair")
	if h.restarts() != 1 {
		t.Fatalf("not restarted after confirmation: %v", h.router.runs)
	}
	needs(t, h.tg.last().text(), "🔧 Сервис перезапущен", "🛡 Мониторинг")
}

func TestMuteSilencesNoticesAndUnmuteRestoresThem(t *testing.T) {
	h := newHarness(t)
	wd := h.withWatchdog(wdConfig())
	h.press(adminID, "cmd:/mute 60")
	needs(t, h.tg.last().text(), "🔕 Уведомления мониторинга выключены на 60 мин", "🔕 Уведомления выключены до")
	if !h.tg.last().hasData("cmd:/unmute") {
		t.Fatalf("no unmute button: %v", h.tg.last().buttons())
	}
	before := h.tg.count("sendMessage")
	h.router.down = true
	wd.Tick(context.Background())
	wd.Tick(context.Background())
	if h.tg.count("sendMessage") != before {
		t.Fatal("a notice was sent while muted")
	}
	if h.restarts() != 1 {
		t.Fatal("muting stopped the repair")
	}
	h.press(adminID, "cmd:/unmute")
	needs(t, h.tg.last().text(), "🔔 Уведомления включены")
	h.say(adminID, "/mute abc")
	needs(t, h.tg.last().text(), "использование: /mute")
}

func TestViewerSeesMonitoringButCannotAct(t *testing.T) {
	h := newHarness(t)
	wd := h.withWatchdog(wdConfig())
	wd.Tick(context.Background())
	h.press(viewerID, "nav:watch")
	c := h.tg.last()
	needs(t, c.text(), "✅ OpenWrt")
	for _, d := range []string{"cmd:/repair", "cmd:/mute 60", "cmd:/unmute"} {
		if c.hasData(d) {
			t.Fatalf("viewer got %s", d)
		}
	}
	h.press(viewerID, "cmd:/repair")
	h.say(viewerID, "/mute 5")
	if h.restarts() != 0 || !wd.MutedUntil().IsZero() {
		t.Fatal("a viewer changed the monitoring")
	}
}

func TestMonitoringOffExplainsItself(t *testing.T) {
	h := newHarness(t)
	h.press(adminID, "nav:watch")
	needs(t, h.tg.last().text(), "Выключен в конфиге")
	h.press(adminID, "cmd:/mute 5")
	needs(t, h.tg.last().text(), "мониторинг выключен")
}
