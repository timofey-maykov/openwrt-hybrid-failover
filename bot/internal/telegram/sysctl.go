package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerctl"
)

// WithShellIDs sets who may use /sh (the config's allow_shell_ids).
func (h CommandHandler) WithShellIDs(ids []int64) CommandHandler {
	h.shellIDs = make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		h.shellIDs[id] = struct{}{}
	}
	return h
}

// ShellAllowed reports whether the user may run /sh.
func (h CommandHandler) ShellAllowed(userID int64) bool {
	_, ok := h.shellIDs[userID]
	return ok
}

// sysCommandNames lists every router-management command, for keyboards and
// for telling them apart from the Hybrid Failover ones.
var sysCommandNames = map[string]bool{
	"/sysinfo": true, "/reboot": true, "/services": true, "/service": true,
	"/syslog": true, "/dmesg": true, "/ping": true, "/traceroute": true,
	"/wan": true, "/ifup": true, "/ifdown": true,
	"/devices": true, "/wol": true, "/kick": true,
	"/wifi": true, "/wifi_on": true, "/wifi_off": true,
	"/portfwd": true, "/portfwd_add": true, "/portfwd_del": true, "/fw_restart": true,
	"/apk_check": true, "/apk_upgrade": true, "/backup": true,
	"/update_check": true, "/update_apply": true, "/slots": true, "/mode5g": true,
	"/sh": true,
}

func isSysCommand(cmd string) bool {
	f := strings.Fields(cmd)
	return len(f) > 0 && sysCommandNames[f[0]]
}

// needsConfirm is true for the router commands that can cut the user off,
// overwrite settings or send secrets out. They are confirmed whether they
// come from a button or are typed.
func needsConfirm(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "/reboot", "/ifdown", "/wifi_off", "/fw_restart", "/portfwd_add", "/portfwd_del",
		"/update_apply", "/apk_upgrade", "/backup", "/sh":
		return true
	case "/service":
		if len(fields) >= 3 {
			switch fields[2] {
			case "stop", "restart", "disable":
				return true
			}
		}
	case "/mode5g":
		return len(fields) >= 2
	}
	return false
}

// confirmPrompt says in plain words what the pending command will do.
func confirmPrompt(text string) string {
	f := strings.Fields(text)
	var what string
	switch f[0] {
	case "/reboot":
		what = "Роутер перезагрузится, сеть пропадёт примерно на минуту."
	case "/ifdown":
		what = "Интерфейс будет выключен. Если связь с вами идёт через него, роутер станет недоступен."
	case "/wifi_off":
		what = "Wi-Fi будет выключен и останется выключенным после перезагрузки."
	case "/fw_restart":
		what = "Файрвол перезапустится, соединения на секунды прервутся."
	case "/portfwd_add":
		what = "Порт роутера будет открыт наружу и проброшен в локальную сеть."
	case "/portfwd_del":
		what = "Проброс будет удалён, файрвол перезагрузится."
	case "/update_apply":
		what = "Будет скачана и установлена новая прошивка, роутер перезагрузится и пропадёт на несколько минут."
	case "/apk_upgrade":
		what = "Обновятся все пакеты, службы перезапустятся. Если обновится сам бот, он перезапустится."
	case "/backup":
		what = "В этот чат придёт архив настроек. В нём пароли Wi-Fi, ключи и токен бота, храните его как секрет."
	case "/service":
		what = "Служба будет остановлена, перезапущена или отключена."
	case "/mode5g":
		what = "Сменится режим 5 ГГц, сеть 5 ГГц пропадёт примерно на полминуты."
	case "/sh":
		what = "Команда выполнится на роутере от root."
	}
	return "Подтвердите действие (30 секунд):\n" + text + "\n\n" + what
}

func (h CommandHandler) instanceFor(userID int64) (routerctl.Service, error) {
	inst, err := h.mgr.InstanceFor(userID)
	if err != nil {
		return routerctl.Service{}, err
	}
	return inst.Ctl, nil
}

func optInt(fields []string, i int) int {
	if len(fields) > i {
		if n, err := strconv.Atoi(fields[i]); err == nil {
			return n
		}
	}
	return 0
}

// dispatchRouter handles the router-management commands. handled is false for
// any other command.
func (h CommandHandler) dispatchRouter(ctx context.Context, userID int64, fields []string) (resp string, handled bool, err error) {
	if !sysCommandNames[fields[0]] {
		return "", false, nil
	}
	ctl, err := h.instanceFor(userID)
	if err != nil {
		return "", true, err
	}
	need := func(n int, usage string) error {
		if len(fields) < n {
			return fmt.Errorf("использование: %s", usage)
		}
		return nil
	}
	switch fields[0] {
	case "/sysinfo":
		resp, err = ctl.SysInfo(ctx)
	case "/reboot":
		resp, err = ctl.Reboot(ctx)
	case "/services":
		resp, err = ctl.Services(ctx)
	case "/service":
		if err = need(3, "/service <имя> start|stop|restart|reload|enable|disable"); err == nil {
			resp, err = ctl.ServiceAction(ctx, fields[1], fields[2])
		}
	case "/syslog":
		resp, err = ctl.Syslog(ctx, optInt(fields, 1))
	case "/dmesg":
		resp, err = ctl.Dmesg(ctx, optInt(fields, 1))
	case "/ping":
		if err = need(2, "/ping <адрес>"); err == nil {
			resp, err = ctl.Ping(ctx, fields[1])
		}
	case "/traceroute":
		if err = need(2, "/traceroute <адрес>"); err == nil {
			resp, err = ctl.Traceroute(ctx, fields[1])
		}
	case "/wan":
		resp, err = ctl.WAN(ctx)
	case "/ifup", "/ifdown":
		if err = need(2, fields[0]+" <имя интерфейса>"); err == nil {
			resp, err = ctl.IfUpDown(ctx, fields[1], fields[0] == "/ifup")
		}
	case "/devices":
		resp, err = ctl.Devices(ctx)
	case "/wol":
		if err = need(2, "/wol <mac|имя>"); err == nil {
			resp, err = ctl.WakeOnLAN(ctx, fields[1])
		}
	case "/kick":
		if err = need(2, "/kick <mac|имя>"); err == nil {
			resp, err = ctl.Kick(ctx, fields[1])
		}
	case "/wifi":
		resp, err = ctl.Wifi(ctx)
	case "/wifi_on", "/wifi_off":
		radio := ""
		if len(fields) > 1 {
			radio = fields[1]
		}
		resp, err = ctl.WifiSet(ctx, radio, fields[0] == "/wifi_on")
	case "/portfwd":
		resp, err = ctl.PortFwdList(ctx)
	case "/portfwd_add":
		if err = need(4, "/portfwd_add <tcp|udp|tcpudp> <порт> <ip> [порт_назначения] [имя]"); err == nil {
			destPort, name := "", ""
			if len(fields) > 4 {
				destPort = fields[4]
			}
			if len(fields) > 5 {
				name = fields[5]
			}
			resp, err = ctl.PortFwdAdd(ctx, fields[1], fields[2], fields[3], destPort, name)
		}
	case "/portfwd_del":
		if err = need(2, "/portfwd_del <номер|имя>"); err == nil {
			resp, err = ctl.PortFwdDel(ctx, fields[1])
		}
	case "/fw_restart":
		resp, err = ctl.FirewallRestart(ctx)
	case "/apk_check":
		resp, err = ctl.APKCheck(ctx)
	case "/apk_upgrade":
		resp, err = ctl.APKUpgrade(ctx)
	case "/backup":
		err = fmt.Errorf("архив присылается самим ботом, нажмите кнопку или отправьте команду из чата")
	case "/update_check":
		resp, err = ctl.BeamUpdateCheck(ctx)
	case "/update_apply":
		resp, err = ctl.BeamUpdateApply(ctx)
	case "/slots":
		resp, err = ctl.Slots(ctx)
	case "/mode5g":
		mode := ""
		if len(fields) > 1 {
			mode = fields[1]
		}
		resp, err = ctl.Mode5G(ctx, mode)
	case "/sh":
		err = fmt.Errorf("использование: /sh <команда>")
	}
	return resp, true, err
}

// shell runs /sh. The command line is the rest of the original message, not
// the whitespace-split fields, so quoting survives.
func (h CommandHandler) shell(ctx context.Context, userID int64, text string) (string, error) {
	if !h.ShellAllowed(userID) {
		return "", fmt.Errorf("/sh выключена: чтобы включить, добавьте свой ID в allow_shell_ids в /etc/hybrid-failover-bot.json")
	}
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "/sh"))
	if rest == "" {
		return "", fmt.Errorf("использование: /sh <команда>")
	}
	ctl, err := h.instanceFor(userID)
	if err != nil {
		return "", err
	}
	return ctl.Shell(ctx, rest)
}

// Backup returns the configuration archive and a file name for it.
func (h CommandHandler) Backup(ctx context.Context, userID int64) (string, []byte, error) {
	ctl, err := h.instanceFor(userID)
	if err != nil {
		return "", nil, err
	}
	data, err := ctl.Backup(ctx)
	if err != nil {
		return "", nil, err
	}
	name := "router"
	if inst, ierr := h.mgr.InstanceFor(userID); ierr == nil && inst.Name != "" {
		name = strings.Map(func(r rune) rune {
			if r == ' ' || r == '/' || r == '\\' {
				return '_'
			}
			return r
		}, inst.Name)
	}
	return fmt.Sprintf("backup-%s-%s.tar.gz", name, time.Now().Format("20060102-150405")), data, nil
}

// routerHelpLines is the router-management part of /help.
func routerHelpLines() []string {
	return []string{
		"Управление роутером:",
		"/sysinfo — аптайм, нагрузка, память, место, температура",
		"/wan — интерфейсы, адреса, шлюз, скорость линка",
		"/devices — устройства в сети (DHCP и Wi-Fi)",
		"/wifi — радио и число клиентов",
		"/wifi_on [радио], /wifi_off [радио]",
		"/kick <mac|имя> — отключить клиента от Wi-Fi",
		"/wol <mac|имя> — разбудить устройство",
		"/ifup <имя>, /ifdown <имя>",
		"/services — список служб",
		"/service <имя> start|stop|restart|reload|enable|disable",
		"/portfwd — проброс портов",
		"/portfwd_add <tcp|udp|tcpudp> <порт> <ip> [порт_назначения] [имя]",
		"/portfwd_del <номер|имя>",
		"/fw_restart",
		"/syslog [N], /dmesg [N]",
		"/ping <адрес>, /traceroute <адрес>",
		"/apk_check, /apk_upgrade — обновление пакетов",
		"/backup — прислать архив настроек",
		"/reboot",
		"Только на Beam WRT: /update_check, /update_apply, /slots, /mode5g [single|split|mlo]",
		"/sh <команда> — только для ID из allow_shell_ids",
		"Опасные действия просят подтверждения на 30 секунд.",
	}
}

// Screen renders a navigation screen as plain text with its buttons listed,
// for checking the panels from a shell without Telegram.
func (h CommandHandler) Screen(ctx context.Context, userID int64, nav string) (string, bool) {
	sc, ok := h.render(ctx, userID, true, nav)
	if !ok {
		return "", false
	}
	var b strings.Builder
	b.WriteString(sc.text)
	b.WriteString("\n\n[кнопки]\n")
	for _, row := range sc.kb.InlineKeyboard {
		var parts []string
		for _, bt := range row {
			data := ""
			if bt.CallbackData != nil {
				data = *bt.CallbackData
			}
			parts = append(parts, bt.Text+" => "+data)
		}
		b.WriteString(strings.Join(parts, "  |  ") + "\n")
	}
	return b.String(), true
}
