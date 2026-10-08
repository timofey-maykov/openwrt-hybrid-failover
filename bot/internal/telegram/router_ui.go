package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routerctl"
)

// Screens of the "🖥 Роутер" section: the router itself (system, network,
// Wi-Fi, services, port forwards, updates) with a button next to every item.
// Their data comes from routerctl, so they work on local and SSH routers.

func isRouterNav(name string) bool { return name == "router" || strings.HasPrefix(name, "rt_") }

// routerNavAfter names the screen a router command returns to once it ran.
func routerNavAfter(f []string) (string, bool) {
	switch f[0] {
	case "/service":
		if len(f) >= 2 {
			return "rt_s:" + f[1], true
		}
	case "/ifup", "/ifdown":
		return "rt_wan", true
	case "/wifi_on", "/wifi_off":
		return "rt_wifi", true
	case "/portfwd_add", "/portfwd_del", "/fw_restart":
		return "rt_fw", true
	case "/kick", "/wol":
		return "rt_dev", true
	case "/apk_upgrade":
		return "rt_upd", true
	}
	return "", false
}

func routerFail(title string, err error, back string) screen {
	return newScreen(title+"\n\n⚠ "+short(err.Error(), 300), buttonRow{btn("🔄 Повторить", "nav:"+back)}, navRow("router"))
}

func (h CommandHandler) routerScreen(ctx context.Context, userID int64, admin bool, name, arg string) screen {
	ctl, err := h.instanceFor(userID)
	if err != nil {
		return newScreen("⚠ "+err.Error(), navRow("main"))
	}
	switch name {
	case "router":
		return h.routerMenu(ctx, userID, ctl, admin)
	case "rt_info":
		text, err := ctl.SysInfo(ctx)
		if err != nil {
			return routerFail("📊 Состояние", err, "rt_info")
		}
		return newScreen("📊 Состояние\n\n"+text, buttonRow{btn("🔄 Обновить", "nav:rt_info")}, navRow("router"))
	case "rt_wan":
		return routerWAN(ctx, ctl, admin)
	case "rt_dev":
		page, _ := strconv.Atoi(arg)
		return routerDevices(ctx, ctl, page)
	case "rt_d":
		return routerDevice(ctx, ctl, admin, arg)
	case "rt_wifi":
		return routerWifi(ctx, ctl, admin)
	case "rt_svc":
		page, _ := strconv.Atoi(arg)
		return routerServices(ctx, ctl, page)
	case "rt_s":
		return routerService(ctx, ctl, admin, arg)
	case "rt_fw":
		return routerFirewall(ctx, ctl, admin)
	case "rt_log":
		return routerLogs(admin)
	case "rt_upd":
		return routerUpdates(ctx, ctl, admin)
	}
	return h.routerMenu(ctx, userID, ctl, admin)
}

func (h CommandHandler) routerMenu(ctx context.Context, userID int64, ctl routerctl.Service, admin bool) screen {
	title := "🖥 Роутер"
	if inst, err := h.mgr.InstanceFor(userID); err == nil && inst.Name != "" {
		title += " · " + inst.Name
	}
	rows := []buttonRow{
		{btn("📊 Состояние", "nav:rt_info"), btn("🌐 Интернет", "nav:rt_wan")},
		{btn("📱 Устройства", "nav:rt_dev"), btn("📶 Wi-Fi", "nav:rt_wifi")},
		{btn("⚙️ Службы", "nav:rt_svc"), btn("🔀 Проброс портов", "nav:rt_fw")},
		{btn("📜 Журналы", "nav:rt_log"), btn("📦 Обновления", "nav:rt_upd")},
	}
	if admin {
		rows = append(rows, buttonRow{btn("💾 Архив настроек", "cmd:/backup"), btn("♻️ Перезагрузка", "cmd:/reboot")})
	}
	if _, slots, split := ctl.HasBeam(ctx); slots || split {
		var beam buttonRow
		if slots {
			beam = append(beam, btn("🧬 Слоты", "cmd:/slots"))
		}
		if split && admin {
			beam = append(beam, btn("📡 Режим 5 ГГц", "cmd:/mode5g"))
		}
		if len(beam) > 0 {
			rows = append(rows, beam)
		}
	}
	rows = append(rows, navRow("main"))
	return newScreen(title+"\n\nСостояние, сеть, Wi-Fi, службы и обслуживание самого роутера.", rows...)
}

func routerWAN(ctx context.Context, ctl routerctl.Service, admin bool) screen {
	text, err := ctl.WAN(ctx)
	if err != nil {
		return routerFail("🌐 Интернет", err, "rt_wan")
	}
	// the hint about /ifup is replaced by the buttons
	if i := strings.Index(text, "\n\nВключить и выключить"); i >= 0 {
		text = text[:i]
	}
	var rows []buttonRow
	if admin {
		if ifs, err := ctl.Interfaces(ctx); err == nil {
			var bs []buttonRow
			var b []tgbotapi.InlineKeyboardButton
			for _, i := range ifs {
				if i.Up {
					b = append(b, btn("⏹ "+i.Name, "cmd:/ifdown "+i.Name))
				} else {
					b = append(b, btn("▶ "+i.Name, "cmd:/ifup "+i.Name))
				}
			}
			bs = pairs(b)
			rows = append(rows, bs...)
		}
	}
	rows = append(rows, buttonRow{btn("🔄 Обновить", "nav:rt_wan")}, navRow("router"))
	return newScreen("🌐 Интернет\n\n"+text, rows...)
}

func deviceLabel(d routerctl.Device) string {
	name := d.Name
	if name == "" {
		name = d.IP
	}
	if name == "" {
		name = d.MAC
	}
	icon := "🖥"
	if d.WiFi != "" {
		icon = "📱"
	}
	label := icon + " " + name
	if d.Name != "" && d.IP != "" {
		label += " · " + d.IP
	}
	return short(label, 42)
}

func routerDevices(ctx context.Context, ctl routerctl.Service, page int) screen {
	devs, err := ctl.DeviceList(ctx)
	if err != nil {
		return routerFail("📱 Устройства", err, "rt_dev")
	}
	if len(devs) == 0 {
		return newScreen("📱 Устройства\n\nУстройств не найдено.", buttonRow{btn("🔄 Обновить", "nav:rt_dev")}, navRow("router"))
	}
	wifi := 0
	for _, d := range devs {
		if d.WiFi != "" {
			wifi++
		}
	}
	pages := (len(devs) + listsPerPage - 1) / listsPerPage
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	var rows []buttonRow
	for _, d := range devs[page*listsPerPage : min(len(devs), (page+1)*listsPerPage)] {
		rows = append(rows, buttonRow{btn(deviceLabel(d), "nav:rt_d:"+d.MAC)})
	}
	var pr buttonRow
	if page > 0 {
		pr = append(pr, btn("◀ Назад", fmt.Sprintf("nav:rt_dev:%d", page-1)))
	}
	if page < pages-1 {
		pr = append(pr, btn("Вперёд ▶", fmt.Sprintf("nav:rt_dev:%d", page+1)))
	}
	if len(pr) > 0 {
		rows = append(rows, pr)
	}
	rows = append(rows, buttonRow{btn("🔄 Обновить", fmt.Sprintf("nav:rt_dev:%d", page))}, navRow("router"))
	text := fmt.Sprintf("📱 Устройства: %d, по Wi-Fi %d\n\n📱 Wi-Fi, 🖥 остальные. Выберите устройство.", len(devs), wifi)
	if pages > 1 {
		text += fmt.Sprintf("\nСтраница %d из %d.", page+1, pages)
	}
	return newScreen(text, rows...)
}

func routerDevice(ctx context.Context, ctl routerctl.Service, admin bool, mac string) screen {
	devs, err := ctl.DeviceList(ctx)
	if err != nil {
		return routerFail("📱 Устройство", err, "rt_dev")
	}
	for _, d := range devs {
		if d.MAC != mac {
			continue
		}
		lines := []string{"📱 " + deviceLabelPlain(d), ""}
		if d.Name != "" {
			lines = append(lines, "Имя: "+d.Name)
		}
		if d.IP != "" {
			lines = append(lines, "Адрес: "+d.IP)
		}
		lines = append(lines, "MAC: "+d.MAC)
		if d.WiFi != "" {
			lines = append(lines, fmt.Sprintf("Wi-Fi: %s, сигнал %.0f дБм", d.WiFi, d.Signal))
		} else {
			lines = append(lines, "Wi-Fi: не подключено (проводное или не в сети)")
		}
		var rows []buttonRow
		if admin {
			var acts buttonRow
			if d.WiFi != "" {
				acts = append(acts, btn("🚫 Отключить от Wi-Fi", "cmd:/kick "+d.MAC))
			}
			acts = append(acts, btn("⏰ Разбудить", "cmd:/wol "+d.MAC))
			rows = append(rows, acts)
			if d.IP != "" {
				rows = append(rows, buttonRow{btn("🏓 Ping", "cmd:/ping "+d.IP)})
			}
		}
		rows = append(rows, navRow("rt_dev"))
		return newScreen(strings.Join(lines, "\n"), rows...)
	}
	return newScreen("Устройство не найдено, возможно оно ушло из сети.", navRow("rt_dev"))
}

func deviceLabelPlain(d routerctl.Device) string {
	switch {
	case d.Name != "":
		return d.Name
	case d.IP != "":
		return d.IP
	}
	return d.MAC
}

func routerWifi(ctx context.Context, ctl routerctl.Service, admin bool) screen {
	text, err := ctl.Wifi(ctx)
	if err != nil {
		return routerFail("📶 Wi-Fi", err, "rt_wifi")
	}
	if i := strings.Index(text, "\n\nВключить и выключить"); i >= 0 {
		text = text[:i]
	}
	var rows []buttonRow
	if admin {
		if rs, err := ctl.Radios(ctx); err == nil {
			var b []tgbotapi.InlineKeyboardButton
			for _, r := range rs {
				if r.Up {
					b = append(b, btn("📴 "+r.Name+" выкл", "cmd:/wifi_off "+r.Name))
				} else {
					b = append(b, btn("📶 "+r.Name+" вкл", "cmd:/wifi_on "+r.Name))
				}
			}
			rows = append(rows, pairs(b)...)
			if len(rs) > 1 {
				rows = append(rows, buttonRow{btn("⛔ Выключить всё", "cmd:/wifi_off"), btn("✅ Включить всё", "cmd:/wifi_on")})
			}
		}
	}
	rows = append(rows, buttonRow{btn("🔄 Обновить", "nav:rt_wifi")}, navRow("router"))
	return newScreen("📶 Wi-Fi\n\n"+text, rows...)
}

func routerServices(ctx context.Context, ctl routerctl.Service, page int) screen {
	svcs, err := ctl.ServiceList(ctx)
	if err != nil {
		return routerFail("⚙️ Службы", err, "rt_svc")
	}
	running := 0
	for _, s := range svcs {
		if s.Running {
			running++
		}
	}
	pages := (len(svcs) + listsPerPage - 1) / listsPerPage
	if pages == 0 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	var rows []buttonRow
	var b []tgbotapi.InlineKeyboardButton
	for _, s := range svcs[min(len(svcs), page*listsPerPage):min(len(svcs), (page+1)*listsPerPage)] {
		mark := "○"
		if s.Running {
			mark = "●"
		}
		label := mark + " " + s.Name
		if s.Enabled {
			label += " ⚡"
		}
		b = append(b, btn(short(label, 30), "nav:rt_s:"+s.Name))
	}
	rows = append(rows, pairs(b)...)
	var pr buttonRow
	if page > 0 {
		pr = append(pr, btn("◀ Назад", fmt.Sprintf("nav:rt_svc:%d", page-1)))
	}
	if page < pages-1 {
		pr = append(pr, btn("Вперёд ▶", fmt.Sprintf("nav:rt_svc:%d", page+1)))
	}
	if len(pr) > 0 {
		rows = append(rows, pr)
	}
	rows = append(rows, navRow("router"))
	text := fmt.Sprintf("⚙️ Службы: %d, работает %d\n\n● работает, ○ нет данных о работе, ⚡ запускается при загрузке.", len(svcs), running)
	if pages > 1 {
		text += fmt.Sprintf("\nСтраница %d из %d.", page+1, pages)
	}
	return newScreen(text, rows...)
}

func routerService(ctx context.Context, ctl routerctl.Service, admin bool, name string) screen {
	svcs, err := ctl.ServiceList(ctx)
	if err != nil {
		return routerFail("⚙️ Служба", err, "rt_svc")
	}
	for _, s := range svcs {
		if s.Name != name {
			continue
		}
		state := "нет данных о работе"
		if s.Running {
			state = "работает"
		}
		auto := "нет"
		if s.Enabled {
			auto = "да"
		}
		text := fmt.Sprintf("⚙️ %s\n\nСостояние: %s\nАвтозапуск: %s", s.Name, state, auto)
		var rows []buttonRow
		if admin {
			rows = append(rows, buttonRow{btn("▶ Запустить", "cmd:/service "+s.Name+" start")})
			if routerctl.SelfService(s.Name) {
				text += "\n\nСам бот нельзя остановить или перезапустить отсюда."
			} else {
				rows = append(rows, buttonRow{
					btn("⏹ Остановить", "cmd:/service "+s.Name+" stop"),
					btn("🔁 Перезапустить", "cmd:/service "+s.Name+" restart"),
				})
			}
			if s.Enabled {
				rows = append(rows, buttonRow{btn("⚡ Убрать из автозапуска", "cmd:/service "+s.Name+" disable")})
			} else {
				rows = append(rows, buttonRow{btn("⚡ В автозапуск", "cmd:/service "+s.Name+" enable")})
			}
		}
		rows = append(rows, buttonRow{btn("🔄 Обновить", "nav:rt_s:"+s.Name)}, navRow("rt_svc"))
		return newScreen(text, rows...)
	}
	return newScreen("Службы «"+short(name, 40)+"» нет в /etc/init.d.", navRow("rt_svc"))
}

func routerFirewall(ctx context.Context, ctl routerctl.Service, admin bool) screen {
	rules, err := ctl.PortFwdRules(ctx)
	if err != nil {
		return routerFail("🔀 Проброс портов", err, "rt_fw")
	}
	lines := []string{"🔀 Проброс портов", ""}
	if len(rules) == 0 {
		lines = append(lines, "Правил нет.")
	}
	var del []tgbotapi.InlineKeyboardButton
	for _, r := range rules {
		name := r.Name
		if name == "" {
			name = "-"
		}
		proto := r.Proto
		if proto == "" {
			proto = "tcp udp"
		}
		state := ""
		if r.Off {
			state = " (выключено)"
		}
		lines = append(lines, fmt.Sprintf("%d. %s  %s  порт %s → %s:%s%s", r.Num, name, proto, r.SrcPort, r.DestIP, r.DestPort, state))
		del = append(del, btn(short(fmt.Sprintf("🗑 %d %s", r.Num, name), 28), "cmd:/portfwd_del "+r.Ref))
	}
	var rows []buttonRow
	if admin {
		rows = append(rows, pairs(del)...)
		rows = append(rows,
			buttonRow{btn("➕ Добавить правило", "input:portfwd_add")},
			buttonRow{btn("🔥 Перезапустить файрвол", "cmd:/fw_restart")},
		)
	}
	rows = append(rows, buttonRow{btn("🔄 Обновить", "nav:rt_fw")}, navRow("router"))
	return newScreen(strings.Join(lines, "\n"), rows...)
}

func routerLogs(admin bool) screen {
	rows := []buttonRow{
		{btn("📜 Журнал 60", "cmd:/syslog 60"), btn("📜 Журнал 200", "cmd:/syslog 200")},
		{btn("🧠 Ядро 60", "cmd:/dmesg 60"), btn("🧠 Ядро 200", "cmd:/dmesg 200")},
	}
	if admin {
		rows = append(rows, buttonRow{btn("🏓 Ping", "input:ping"), btn("🧭 Traceroute", "input:traceroute")})
	}
	rows = append(rows, navRow("router"))
	return newScreen("📜 Журналы и проверка связи\n\nПароли, ключи и токен бота в журналах скрываются.", rows...)
}

func routerUpdates(ctx context.Context, ctl routerctl.Service, admin bool) screen {
	text := "📦 Обновления\n\nПакеты обновляются через apk. Проверка сначала скачивает списки пакетов."
	var rows []buttonRow
	if admin {
		rows = append(rows, buttonRow{btn("🔍 Проверить пакеты", "cmd:/apk_check"), btn("⬆ Обновить все", "cmd:/apk_upgrade")})
		if upd, _, _ := ctl.HasBeam(ctx); upd {
			text += "\n\nПрошивка Beam WRT обновляется отдельно."
			rows = append(rows, buttonRow{btn("🧬 Проверить прошивку", "cmd:/update_check"), btn("⬇ Установить прошивку", "cmd:/update_apply")})
		}
	}
	rows = append(rows, navRow("router"))
	return newScreen(text, rows...)
}
