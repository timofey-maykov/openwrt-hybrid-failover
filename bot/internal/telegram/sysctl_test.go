package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/config"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
)

func TestNeedsConfirm(t *testing.T) {
	yes := []string{
		"/reboot", "/ifdown wan", "/wifi_off", "/wifi_off radio1", "/fw_restart",
		"/portfwd_add tcp 80 192.168.1.5", "/portfwd_del 1", "/update_apply", "/apk_upgrade",
		"/backup", "/sh ls", "/service dropbear stop", "/service firewall restart",
		"/service foo disable", "/mode5g mlo",
	}
	for _, c := range yes {
		if !needsConfirm(strings.Fields(c)) {
			t.Errorf("%q must need confirmation", c)
		}
	}
	no := []string{
		"/sysinfo", "/wan", "/devices", "/wifi", "/wifi_on", "/ifup wan", "/services",
		"/service foo start", "/service foo enable", "/service foo reload", "/service foo",
		"/portfwd", "/syslog 10", "/ping 8.8.8.8", "/kick aa:bb:cc:dd:ee:ff", "/mode5g",
		"/slots", "/status", "/param_apply", "",
	}
	for _, c := range no {
		if needsConfirm(strings.Fields(c)) {
			t.Errorf("%q must not need confirmation", c)
		}
	}
}

func TestConfirmPromptExplainsEveryDangerousCommand(t *testing.T) {
	for _, c := range []string{"/reboot", "/ifdown wan", "/wifi_off", "/fw_restart", "/portfwd_add tcp 80 192.168.1.5",
		"/portfwd_del 1", "/update_apply", "/apk_upgrade", "/backup", "/service a stop", "/mode5g mlo", "/sh id"} {
		p := confirmPrompt(c)
		if !strings.Contains(p, c) || len(p) < len(c)+30 {
			t.Errorf("prompt for %q says too little: %q", c, p)
		}
	}
}

func TestConfirmTokenIsOneShotPerUserAndExpires(t *testing.T) {
	b := &Bot{pendingConfirm: map[int64]pendingConfirm{}}
	id := b.setConfirmToken(1, "/reboot")
	if _, ok := b.takeConfirmToken(2, id); ok {
		t.Fatal("another user took the confirmation")
	}
	if _, ok := b.takeConfirmToken(1, "deadbeef"); ok {
		t.Fatal("wrong token accepted")
	}
	cmd, ok := b.takeConfirmToken(1, id)
	if !ok || cmd != "/reboot" {
		t.Fatalf("%q %v", cmd, ok)
	}
	if _, ok := b.takeConfirmToken(1, id); ok {
		t.Fatal("token usable twice")
	}
	id = b.setConfirmToken(1, "/reboot")
	b.confirmMu.Lock()
	st := b.pendingConfirm[1]
	st.expiresAt = time.Now().Add(-time.Second)
	b.pendingConfirm[1] = st
	b.confirmMu.Unlock()
	if _, ok := b.takeConfirmToken(1, id); ok {
		t.Fatal("expired token accepted")
	}
}

func TestNewerConfirmReplacesOlder(t *testing.T) {
	b := &Bot{pendingConfirm: map[int64]pendingConfirm{}}
	first := b.setConfirmToken(1, "/reboot")
	second := b.setConfirmToken(1, "/fw_restart")
	if _, ok := b.takeConfirmToken(1, first); ok {
		t.Fatal("old confirmation still valid after a new one")
	}
	if cmd, ok := b.takeConfirmToken(1, second); !ok || cmd != "/fw_restart" {
		t.Fatalf("%q %v", cmd, ok)
	}
}

func TestShellGate(t *testing.T) {
	h := CommandHandler{}.WithShellIDs([]int64{7})
	if !h.ShellAllowed(7) || h.ShellAllowed(8) {
		t.Fatal("shell allowlist")
	}
	if (CommandHandler{}).ShellAllowed(7) {
		t.Fatal("shell must be off by default")
	}
}

func TestSysCommandsKnown(t *testing.T) {
	for _, c := range []string{"/sysinfo", "/sh uptime", "/service a b", "/portfwd_add"} {
		if !isSysCommand(c) {
			t.Errorf("%q not recognised", c)
		}
	}
	for _, c := range []string{"/status", "/param_apply", "", "sysinfo"} {
		if isSysCommand(c) {
			t.Errorf("%q wrongly recognised", c)
		}
	}
}

func TestHelpListsRouterCommands(t *testing.T) {
	help := strings.Join(routerHelpLines(), "\n")
	for c := range sysCommandNames {
		if !strings.Contains(help, c) {
			t.Errorf("/help does not mention %s", c)
		}
	}
}

func TestConfirmIsBoundToTheRouterItWasAskedFor(t *testing.T) {
	m, err := routers.NewManager(config.Config{Routers: []config.RouterConfig{
		{ID: "a", Name: "A", Local: true}, {ID: "b", Name: "B", Local: true},
	}, ClashAPI: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	b := &Bot{pendingConfirm: map[int64]pendingConfirm{}, h: NewCommandHandler(m, botconfig.Store{})}
	if err := m.SetSelected(1, "a"); err != nil {
		t.Fatal(err)
	}
	id := b.setConfirmToken(1, "/reboot")
	if err := m.SetSelected(1, "b"); err != nil {
		t.Fatal(err)
	}
	if _, ok := b.takeConfirmToken(1, id); ok {
		t.Fatal("/reboot confirmed for router A ran after switching to router B")
	}
	id = b.setConfirmToken(1, "/reboot")
	if cmd, ok := b.takeConfirmToken(1, id); !ok || cmd != "/reboot" {
		t.Fatalf("same router must work: %q %v", cmd, ok)
	}
	if got := b.routerPrefix(1); got != "[B] " {
		t.Fatalf("prefix %q", got)
	}
}
