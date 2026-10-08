package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/audit"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerctl"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/security"
)

const testToken = "1234567890:AAZZexample_fake-token_for-tests_0000"

// ctlFake is the router as the router-management screens see it: it answers
// by the start of "name arg arg" and records every command it was given.
type ctlFake struct {
	mu    sync.Mutex
	calls []string
}

func (f *ctlFake) did(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (f *ctlFake) Run(ctx context.Context, name string, args ...string) (string, error) {
	line := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.mu.Lock()
	f.calls = append(f.calls, line)
	f.mu.Unlock()
	switch {
	case strings.HasPrefix(line, "ubus call system board"):
		return `{"model":"Xiaomi Mi Router AX3000T","hostname":"OpenWrt","kernel":"6.12.94","release":{"description":"OpenWrt 25.12.5"}}`, nil
	case strings.HasPrefix(line, "ubus call system info"):
		return `{"uptime":3600,"load":[9536,17728,27360],"memory":{"total":244760576,"available":79364096}}`, nil
	case strings.HasPrefix(line, "df -k /overlay"):
		return "Filesystem 1K-blocks Used Available Use% Mounted on\n/dev/x 61000 13000 45000 22% /overlay", nil
	case strings.HasPrefix(line, "ubus call network.interface dump"):
		return `{"interface":[{"interface":"loopback","up":true},{"interface":"lan","up":true,"proto":"static","l3_device":"br-lan","uptime":100,"ipv4-address":[{"address":"192.168.42.1","mask":24}]},{"interface":"wan","up":true,"proto":"dhcp","l3_device":"wan","uptime":100,"ipv4-address":[{"address":"192.168.31.79","mask":24}]},{"interface":"vpn","up":false,"proto":"amneziawg"}]}`, nil
	case strings.HasPrefix(line, "ubus call network.device status"):
		return `{"wan":{"speed":"1000F"}}`, nil
	case line == "ubus list":
		return "network\nhostapd.phy0-ap0\nsystem", nil
	case strings.HasPrefix(line, "ubus call hostapd.phy0-ap0 get_clients"):
		return `{"clients":{"aa:bb:cc:dd:ee:01":{"signal":-55}}}`, nil
	case strings.HasPrefix(line, "ubus call hostapd.phy0-ap0 del_client"):
		return "{}", nil
	case strings.HasPrefix(line, "cat /tmp/dhcp.leases"):
		return "1 aa:bb:cc:dd:ee:01 192.168.42.10 phone 01:aa\n2 aa:bb:cc:dd:ee:02 192.168.42.11 desktop 01:bb\n", nil
	case strings.HasPrefix(line, "ubus call network.wireless status"):
		return `{"radio0":{"up":true,"config":{"band":"2g","channel":"1","htmode":"HE20"},"interfaces":[{"ifname":"phy0-ap0","config":{"ssid":"home"}}]},"radio1":{"up":false,"disabled":true,"config":{"band":"5g"},"interfaces":[]}}`, nil
	case strings.HasPrefix(line, "ls /etc/init.d"):
		return "dropbear\nfirewall\nhybrid-failover-bot\n", nil
	case strings.HasPrefix(line, "ls /etc/rc.d"):
		return "S19firewall\nS50dropbear\n", nil
	case strings.HasPrefix(line, "ubus call service list"):
		return `{"firewall":{},"dropbear":{}}`, nil
	case strings.HasPrefix(line, "uci show firewall"):
		return "firewall.@redirect[0]=redirect\nfirewall.@redirect[0].name='web'\nfirewall.@redirect[0].proto='tcp'\nfirewall.@redirect[0].src_dport='8080'\nfirewall.@redirect[0].dest_ip='192.168.42.50'\nfirewall.@redirect[0].dest_port='80'\nfirewall.@redirect[0].target='DNAT'\n", nil
	case strings.HasPrefix(line, "test -x"):
		return "", errors.New("no")
	case strings.HasPrefix(line, "logread -l"):
		return "Oct 8 daemon.err bot: Post https://api.telegram.org/bot" + testToken + "/getUpdates failed", nil
	case strings.HasPrefix(line, "sysupgrade -b"), strings.HasPrefix(line, "rm -f"),
		strings.HasPrefix(line, "reboot"), strings.HasPrefix(line, "ifdown"), strings.HasPrefix(line, "ifup"),
		strings.HasPrefix(line, "uci "), strings.HasPrefix(line, "/etc/init.d/"), strings.HasPrefix(line, "wifi reload"):
		return "", nil
	case strings.HasPrefix(line, "sh -c"):
		return "shell says hi", nil
	}
	return "", errors.New("no answer for: " + line)
}

func (f *ctlFake) RunBytes(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "cat" && len(args) == 1 && args[0] == "/tmp/hf-bot-backup.tar.gz" {
		f.Run(ctx, name, args...)
		return append([]byte{0x1f, 0x8b}, make([]byte, 300)...), nil
	}
	out, err := f.Run(ctx, name, args...)
	return []byte(out), err
}

func (f *ctlFake) RunCoreRPC(ctx context.Context, method string, args ...string) (string, error) {
	return "", errors.New("unused")
}

// routerHarness is the e2e harness with a router that can be managed.
func routerHarness(t *testing.T, shellIDs ...int64) (*harness, *ctlFake) {
	t.Helper()
	tg := newFakeTelegram(t)
	api, err := tgbotapi.NewBotAPIWithAPIEndpoint("TOKEN", tg.srv.URL+"/bot%s/%s")
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRouter{}
	fx := &ctlFake{}
	svc := routing.NewService(fr, "http://127.0.0.1:1", "/etc/init.d/hybrid-failover", "", "glob", 5*time.Second)
	mgr := routers.NewStatic(routers.Instance{ID: "local", Name: "OpenWrt", Service: svc, Ctl: routerctl.New(fx)})
	h := NewCommandHandler(mgr, botconfig.Store{}).WithShellIDs(shellIDs)
	b := New(api, security.NewAuthorizer([]int64{adminID}, []int64{viewerID}),
		audit.New(filepath.Join(t.TempDir(), "audit.log")), h, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &harness{t: t, bot: b, tg: tg, router: fr}, fx
}

// confirmData is the callback of the confirm button on the newest message.
func (h *harness) confirmData() string {
	h.t.Helper()
	d := h.tg.last().buttons()["✅ Подтвердить"]
	if !strings.HasPrefix(d, "cfm:") {
		h.t.Fatalf("no confirmation on the last message: %q %v", h.tg.last().text(), h.tg.last().buttons())
	}
	return d
}

func TestMainMenuLinksToTheRouterSection(t *testing.T) {
	h, _ := routerHarness(t)
	h.say(adminID, "/menu")
	if !h.tg.last().hasData("nav:router") {
		t.Fatalf("main menu lacks the router section: %v", h.tg.last().buttons())
	}
	h.say(adminID, "/manage")
	c := h.tg.last()
	needs(t, c.text(), "Роутер · OpenWrt")
	for _, d := range []string{"nav:rt_info", "nav:rt_wan", "nav:rt_dev", "nav:rt_wifi", "nav:rt_svc", "nav:rt_fw", "nav:rt_log", "nav:rt_upd", "cmd:/backup", "cmd:/reboot"} {
		if !c.hasData(d) {
			t.Fatalf("router menu lacks %s: %v", d, c.buttons())
		}
	}
}

func TestRouterScreensShowRealData(t *testing.T) {
	h, _ := routerHarness(t)
	h.press(adminID, "nav:rt_info")
	needs(t, h.tg.last().text(), "Xiaomi Mi Router AX3000T", "Аптайм: 1ч 0м", "Нагрузка: 0.15")
	h.press(adminID, "nav:rt_wan")
	needs(t, h.tg.last().text(), "wan (dhcp)", "192.168.31.79/24", "линк 1000F")
	h.press(adminID, "nav:rt_wifi")
	needs(t, h.tg.last().text(), "radio0: работает", "сеть home, клиентов 1", "radio1: выключено")
	h.press(adminID, "nav:rt_fw")
	needs(t, h.tg.last().text(), "web", "порт 8080 → 192.168.42.50:80")
}

func TestEveryRouterButtonFitsTelegramLimit(t *testing.T) {
	h, _ := routerHarness(t)
	for _, nav := range []string{"router", "rt_info", "rt_wan", "rt_dev", "rt_d:aa:bb:cc:dd:ee:01", "rt_wifi", "rt_svc", "rt_s:hybrid-failover-bot", "rt_s:firewall", "rt_fw", "rt_log", "rt_upd"} {
		h.press(adminID, "nav:"+nav)
		for label, data := range h.tg.last().buttons() {
			if len(data) > maxCallbackData {
				t.Fatalf("%s: button %q has %d byte payload", nav, label, len(data))
			}
		}
	}
}

func TestDeviceCardKicksAWifiClient(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "nav:rt_dev")
	c := h.tg.last()
	needs(t, c.text(), "Устройства: 2, по Wi-Fi 1")
	if !c.hasData("nav:rt_d:aa:bb:cc:dd:ee:01") || !c.hasData("nav:rt_d:aa:bb:cc:dd:ee:02") {
		t.Fatalf("device buttons: %v", c.buttons())
	}
	h.press(adminID, "nav:rt_d:aa:bb:cc:dd:ee:01")
	c = h.tg.last()
	needs(t, c.text(), "phone", "192.168.42.10", "сигнал -55")
	if !c.hasData("cmd:/kick aa:bb:cc:dd:ee:01") {
		t.Fatalf("no kick button: %v", c.buttons())
	}
	h.press(adminID, "cmd:/kick aa:bb:cc:dd:ee:01")
	if !fx.did(`ubus call hostapd.phy0-ap0 del_client {"addr":"aa:bb:cc:dd:ee:01"`) {
		t.Fatalf("kick never reached the router: %v", fx.calls)
	}
	needs(t, h.tg.last().text(), "отключён от Wi-Fi", "Устройства")

	// a wired device has no kick button
	h.press(adminID, "nav:rt_d:aa:bb:cc:dd:ee:02")
	if h.tg.last().hasData("cmd:/kick aa:bb:cc:dd:ee:02") {
		t.Fatal("kick offered for a device that is not on Wi-Fi")
	}
}

func TestDangerousButtonAsksThenActsOnce(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "cmd:/ifdown wan")
	needs(t, h.tg.last().text(), "Подтвердите", "/ifdown wan", "Интерфейс будет выключен")
	if fx.did("ifdown") {
		t.Fatal("ran before the confirmation")
	}
	data := h.confirmData()
	h.press(adminID, data)
	if !fx.did("ifdown wan") {
		t.Fatalf("not run after confirming: %v", fx.calls)
	}
	needs(t, h.tg.last().text(), "Интернет")
	n := 0
	for _, c := range fx.calls {
		if c == "ifdown wan" {
			n++
		}
	}
	h.press(adminID, data) // second press of the same button
	for _, c := range fx.calls {
		if c == "ifdown wan" {
			n--
		}
	}
	if n != 0 {
		t.Fatal("the confirmation worked twice")
	}
}

func TestTypedRebootNeedsConfirmation(t *testing.T) {
	h, fx := routerHarness(t)
	h.say(adminID, "/reboot")
	needs(t, h.tg.last().text(), "Подтвердите", "перезагрузится")
	if fx.did("reboot") {
		t.Fatal("rebooted without asking")
	}
	real := h.confirmData()
	h.press(adminID, "cfm:deadbeefdead")
	if fx.did("reboot") {
		t.Fatal("a made-up confirmation was accepted")
	}
	h.press(adminID, real) // a wrong guess must not cancel the real question
	if !fx.did("reboot") {
		t.Fatalf("reboot not run after confirming: %v", fx.calls)
	}
}

func TestConfirmationBelongsToTheAskingUser(t *testing.T) {
	h, fx := routerHarness(t)
	h.say(adminID, "/reboot")
	data := h.confirmData()
	h.press(viewerID, data)
	if fx.did("reboot") {
		t.Fatal("a viewer confirmed the admin's reboot")
	}
}

func TestViewerSeesRouterStateButChangesNothing(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(viewerID, "nav:router")
	for label, data := range h.tg.last().buttons() {
		if data == "cmd:/reboot" || data == "cmd:/backup" {
			t.Fatalf("viewer got %q", label)
		}
	}
	for _, nav := range []string{"rt_svc", "rt_s:firewall", "rt_fw", "rt_wifi", "rt_wan", "rt_d:aa:bb:cc:dd:ee:01"} {
		h.press(viewerID, "nav:"+nav)
		for label, data := range h.tg.last().buttons() {
			if strings.HasPrefix(data, "cmd:/service") || strings.HasPrefix(data, "cmd:/portfwd") || strings.HasPrefix(data, "cmd:/wifi") ||
				strings.HasPrefix(data, "cmd:/if") || strings.HasPrefix(data, "cmd:/kick") || strings.HasPrefix(data, "input:") {
				t.Fatalf("%s: viewer got write button %q (%s)", nav, label, data)
			}
		}
	}
	h.press(viewerID, "cmd:/reboot")
	h.say(viewerID, "/reboot")
	needs(t, h.tg.last().text(), "Доступ запрещен")
	if fx.did("reboot") {
		t.Fatal("a viewer rebooted the router")
	}
	h.say(viewerID, "/sysinfo")
	needs(t, h.tg.last().text(), "Аптайм")
}

func TestBotServiceCannotBeStoppedFromItsOwnScreen(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "nav:rt_s:hybrid-failover-bot")
	c := h.tg.last()
	for _, act := range []string{"stop", "restart"} {
		if c.hasData("cmd:/service hybrid-failover-bot " + act) {
			t.Fatalf("offered %s for the bot itself", act)
		}
	}
	h.say(adminID, "/service hybrid-failover-bot restart")
	h.press(adminID, h.confirmData())
	if fx.did("/etc/init.d/hybrid-failover-bot restart") {
		t.Fatal("the bot restarted itself")
	}
	needs(t, h.tg.last().text(), "не может остановить или перезапустить сам себя")

	h.press(adminID, "nav:rt_s:firewall")
	for _, act := range []string{"start", "stop", "restart", "disable"} {
		if !h.tg.last().hasData("cmd:/service firewall " + act) {
			t.Fatalf("firewall screen lacks %s: %v", act, h.tg.last().buttons())
		}
	}
}

func TestPortForwardDeleteGoesThroughConfirmation(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "nav:rt_fw")
	if !h.tg.last().hasData("cmd:/portfwd_del web") || !h.tg.last().hasData("input:portfwd_add") {
		t.Fatalf("buttons: %v", h.tg.last().buttons())
	}
	h.press(adminID, "cmd:/portfwd_del web")
	if fx.did("uci delete") {
		t.Fatal("deleted before the confirmation")
	}
	h.press(adminID, h.confirmData())
	if !fx.did("uci delete firewall.@redirect[0]") || !fx.did("/etc/init.d/firewall reload") {
		t.Fatalf("calls: %v", fx.calls)
	}
}

func TestAddingAPortForwardByTypingNeedsConfirmation(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "input:portfwd_add")
	needs(t, h.tg.last().text(), "<tcp|udp|tcpudp>")
	h.say(adminID, "tcp 2222 192.168.42.60 22 ssh")
	needs(t, h.tg.last().text(), "Подтвердите", "/portfwd_add tcp 2222 192.168.42.60 22 ssh")
	if fx.did("uci add") {
		t.Fatal("added before the confirmation")
	}
}

func TestShellIsOffUnlessAllowed(t *testing.T) {
	h, fx := routerHarness(t)
	h.say(adminID, "/sh id")
	needs(t, h.tg.last().text(), "/sh выключена", "allow_shell_ids")
	h.press(adminID, "cmd:/sh id")
	if fx.did("sh -c") {
		t.Fatal("shell ran while switched off")
	}

	h2, fx2 := routerHarness(t, adminID)
	h2.say(adminID, `/sh echo "a   b"`)
	needs(t, h2.tg.last().text(), "Подтвердите", "от root")
	if fx2.did("sh -c") {
		t.Fatal("shell ran before the confirmation")
	}
	h2.press(adminID, h2.confirmData())
	if !fx2.did(`sh -c echo "a   b"`) {
		t.Fatalf("quoting lost: %v", fx2.calls)
	}
}

func TestBackupArrivesAsAFileAfterConfirmation(t *testing.T) {
	h, fx := routerHarness(t)
	h.press(adminID, "cmd:/backup")
	needs(t, h.tg.last().text(), "Подтвердите", "пароли Wi-Fi")
	if h.tg.count("sendDocument") != 0 {
		t.Fatal("sent the archive before the confirmation")
	}
	h.press(adminID, h.confirmData())
	if h.tg.count("sendDocument") != 1 {
		t.Fatalf("archive not sent: %v", h.tg.calls)
	}
	if !fx.did("rm -f /tmp/hf-bot-backup.tar.gz") {
		t.Fatal("archive left on the router")
	}
}

func TestLogsNeverLeakTheBotToken(t *testing.T) {
	h, _ := routerHarness(t)
	h.press(adminID, "cmd:/syslog 60")
	if strings.Contains(h.tg.everything(), "AAZZex") || strings.Contains(h.tg.everything(), "1234567890") {
		t.Fatalf("the bot token reached Telegram:\n%s", h.tg.last().text())
	}
	needs(t, h.tg.last().text(), "getUpdates failed")
}

func TestRouterNavAfter(t *testing.T) {
	for cmd, want := range map[string]string{
		"/service firewall restart":  "rt_s:firewall",
		"/ifdown wan":                "rt_wan",
		"/wifi_off radio1":           "rt_wifi",
		"/portfwd_add tcp 1 2.3.4.5": "rt_fw",
		"/portfwd_del web":           "rt_fw",
		"/kick aa:bb:cc:dd:ee:01":    "rt_dev",
		"/apk_upgrade":               "rt_upd",
	} {
		got, ok := navAfter(cmd)
		if !ok || got != want {
			t.Errorf("navAfter(%q) = %q %v, want %q", cmd, got, ok, want)
		}
	}
	if _, ok := navAfter("/sysinfo"); ok {
		t.Error("/sysinfo has no screen to return to")
	}
}
