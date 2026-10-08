package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/diag"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/listroutes"
)

// Telegram refuses a keyboard if any callback payload is over 64 bytes.
const maxCallbackData = 64

const listsPerPage = 8

type screen struct {
	text string
	kb   tgbotapi.InlineKeyboardMarkup
}

type buttonRow = []tgbotapi.InlineKeyboardButton

func btn(label, data string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(label, data)
}

func newScreen(text string, rows ...buttonRow) screen {
	kept := make([]buttonRow, 0, len(rows))
	for _, r := range rows {
		var ok buttonRow
		for _, b := range r {
			if b.CallbackData != nil && len(*b.CallbackData) > maxCallbackData {
				continue
			}
			ok = append(ok, b)
		}
		if len(ok) > 0 {
			kept = append(kept, ok)
		}
	}
	return screen{text: text, kb: tgbotapi.NewInlineKeyboardMarkup(kept...)}
}

// pairs lays buttons out two to a row.
func pairs(bs []tgbotapi.InlineKeyboardButton) []buttonRow {
	var rows []buttonRow
	for i := 0; i < len(bs); i += 2 {
		end := i + 2
		if end > len(bs) {
			end = len(bs)
		}
		rows = append(rows, buttonRow(bs[i:end]))
	}
	return rows
}

func navRow(back string) buttonRow {
	if back == "main" {
		return buttonRow{btn("🏠 Меню", "nav:main")}
	}
	return buttonRow{btn("⬅ Назад", "nav:"+back), btn("🏠 Меню", "nav:main")}
}

func pendingRows(n int, admin bool) []buttonRow {
	if n == 0 || !admin {
		return nil
	}
	return []buttonRow{
		{btn(fmt.Sprintf("✅ Применить (%d)", n), "cmd:/param_apply"), btn("↩️ Откатить", "cmd:/param_rollback")},
		{btn("👁 Что изменится", "cmd:/param_preview")},
	}
}

func pendingLine(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("\n⏳ Изменений в ожидании: %d. Они вступят в силу после «Применить».", n)
}

func short(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func channelName(ch diag.ChannelStatus) string {
	name := ch.Display
	if name == "" {
		name = ch.Name
	}
	return strings.TrimSuffix(name, " ("+ch.Name+")")
}

func channelIcon(ch diag.ChannelStatus) string {
	switch {
	case ch.Type == "urltest":
		return "🤖"
	case ch.Available:
		return "✅"
	case ch.Probed:
		return "❌"
	default:
		return "⚪"
	}
}

func delayText(ms int) string {
	if ms <= 0 {
		return ""
	}
	return fmt.Sprintf(" · %d мс", ms)
}

// render builds the screen for nav, or reports false for panels the bot
// renders itself (params, uci, config).
func (h CommandHandler) render(ctx context.Context, userID int64, admin bool, nav string) (screen, bool) {
	name, arg, _ := strings.Cut(nav, ":")
	switch name {
	case "", "main", "service", "failover", "channels", "channels_probe", "subs", "lists", "lst", "settings", "system", "watch", "watch_check":
	default:
		if !isRouterNav(name) {
			return screen{}, false
		}
	}
	rt, err := h.routingFor(userID)
	if err != nil {
		rows := routerRows(h.mgr, userID)
		return newScreen("⚠ "+err.Error()+"\n\nВыберите роутер:", rows...), true
	}
	if isRouterNav(name) {
		return h.routerScreen(ctx, userID, admin, name, arg), true
	}
	switch name {
	case "channels", "failover":
		return h.channelsScreen(ctx, rt, admin, false), true
	case "channels_probe":
		return h.channelsScreen(ctx, rt, admin, true), true
	case "subs":
		return h.subsScreen(ctx, rt, admin), true
	case "lists":
		page, _ := strconv.Atoi(arg)
		return h.listsScreen(ctx, rt, admin, page), true
	case "lst":
		idx, _ := strconv.Atoi(arg)
		return h.listScreen(ctx, rt, admin, idx), true
	case "settings":
		return h.settingsScreen(ctx, rt, admin), true
	case "system", "service":
		return h.systemScreen(ctx, rt, admin), true
	case "watch", "watch_check":
		return h.watchScreen(ctx, userID, admin, name == "watch_check"), true
	default:
		return h.mainScreen(ctx, userID, rt, admin), true
	}
}

func routerRows(mgr *routers.Manager, userID int64) []buttonRow {
	if mgr == nil || !mgr.Multi() {
		return nil
	}
	var rows []buttonRow
	for _, r := range mgr.List() {
		label := r.Name
		if mgr.SelectedID(userID) == r.ID {
			label = "▶ " + label
		}
		rows = append(rows, buttonRow{btn("🌐 "+label, "cmd:/use "+r.ID)})
	}
	return rows
}

func (h CommandHandler) mainScreen(ctx context.Context, userID int64, rt routing.Service, admin bool) screen {
	title := "🛰 Hybrid Failover"
	if inst, err := h.mgr.InstanceFor(userID); err == nil && inst.Name != "" {
		title += " · " + inst.Name
	}
	lines := []string{title, ""}
	rep, err := rt.StatusReport(ctx)
	if err != nil {
		lines = append(lines, "🔴 Ядро не отвечает: "+short(err.Error(), 160))
	} else {
		lines = append(lines, engineLines(rep)...)
	}
	if line := h.watchLine(); line != "" {
		lines = append(lines, line)
	}
	n := rt.PendingCount(ctx)
	text := strings.Join(lines, "\n") + pendingLine(n)

	rows := routerRows(h.mgr, userID)
	rows = append(rows,
		buttonRow{btn("📡 Каналы", "nav:channels"), btn("📰 Подписки", "nav:subs")},
		buttonRow{btn("🗂 Списки", "nav:lists"), btn("⚙️ Настройки", "nav:settings")},
		buttonRow{btn("🩺 Проверка", "cmd:/health"), btn("🛡 Мониторинг", "nav:watch")},
		buttonRow{btn("🛠 Система", "nav:system"), btn("🖥 Роутер", "nav:router")},
	)
	rows = append(rows, pendingRows(n, admin)...)
	return newScreen(text, rows...)
}

func engineLines(rep diag.Report) []string {
	running := rep.EngineRunning || rep.SingboxRunning
	var lines []string
	switch {
	case running && rep.NFTOK:
		lines = append(lines, "🟢 Движок работает, маршрутизация активна")
	case running:
		lines = append(lines, "🟡 Движок работает, но правила nft не в порядке")
	default:
		lines = append(lines, "🔴 Движок остановлен")
	}
	for _, ch := range rep.Channels {
		if !ch.Selected {
			continue
		}
		line := "📡 Активный: " + channelName(ch) + delayText(ch.DelayMs)
		if ch.Type == "urltest" && rep.Failover != nil && rep.Failover.URLTestNow != "" {
			line += "\n    сейчас через " + nameOf(rep, rep.Failover.URLTestNow)
		}
		lines = append(lines, line)
		break
	}
	up, total := 0, 0
	for _, ch := range rep.Channels {
		if ch.Type == "urltest" {
			continue
		}
		total++
		if ch.Available {
			up++
		}
	}
	if total > 0 {
		lines = append(lines, fmt.Sprintf("🔌 Каналов доступно: %d из %d", up, total))
	}
	if rep.Failover != nil && rep.Failover.Policy != "" {
		lines = append(lines, "🧭 Политика: "+policyLabel(rep.Failover.Policy))
	}
	if len(rep.Errors) > 0 {
		lines = append(lines, "⚠ "+short(strings.Join(rep.Errors, "; "), 200))
	}
	return lines
}

func nameOf(rep diag.Report, tag string) string {
	for _, ch := range rep.Channels {
		if ch.Name == tag {
			return channelName(ch)
		}
	}
	return tag
}

func policyLabel(p string) string {
	switch p {
	case "outage-only":
		return "только при падении"
	case "prefer-primary":
		return "основной в приоритете"
	case "fastest":
		return "самый быстрый"
	}
	return p
}

func (h CommandHandler) channelsScreen(ctx context.Context, rt routing.Service, admin, probe bool) screen {
	var rep diag.Report
	var err error
	if probe {
		rep, err = rt.HealthReport(ctx)
	} else {
		rep, err = rt.StatusReport(ctx)
	}
	if err != nil {
		return newScreen("📡 Каналы\n\n⚠ "+short(err.Error(), 300), buttonRow{btn("🔄 Повторить", "nav:channels")}, navRow("main"))
	}
	head := "📡 Каналы"
	if rep.Failover != nil && rep.Failover.Policy != "" {
		head += " · " + policyLabel(rep.Failover.Policy)
	}
	lines := []string{head, ""}
	var switches []tgbotapi.InlineKeyboardButton
	for _, ch := range rep.Channels {
		mark := "  "
		if ch.Selected {
			mark = "▶ "
		}
		line := mark + channelIcon(ch) + " " + channelName(ch)
		if ch.Type != "" && ch.Type != "urltest" {
			line += " (" + ch.Type + ")"
		}
		line += delayText(ch.DelayMs)
		if !ch.Available && ch.Type != "urltest" {
			switch {
			case ch.Detail != "":
				line += " · " + short(ch.Detail, 40)
			case ch.Probed:
				line += " · нет ответа"
			default:
				line += " · не проверялся"
			}
		}
		if ch.Type == "urltest" && rep.Failover != nil && rep.Failover.URLTestNow != "" {
			line += "\n      сейчас: " + nameOf(rep, rep.Failover.URLTestNow)
		}
		lines = append(lines, line)

		label := channelIcon(ch) + " " + short(channelName(ch), 22)
		if ch.Selected {
			label = "▶ " + short(channelName(ch), 22)
		}
		switches = append(switches, btn(label, "cmd:/switch "+ch.Name))
	}
	if len(rep.Channels) == 0 {
		lines = append(lines, "Каналы не найдены.")
	}
	if probe {
		lines = append(lines, "", "Проверено только что.")
	} else {
		lines = append(lines, "", "Задержки из последней проверки.")
	}
	var rows []buttonRow
	if admin && len(switches) > 0 {
		lines = append(lines, "Нажмите на канал, чтобы переключиться на него.")
		rows = append(rows, pairs(switches)...)
	}
	rows = append(rows, buttonRow{btn("🔄 Проверить сейчас", "nav:channels_probe")}, navRow("main"))
	return newScreen(strings.Join(lines, "\n"), rows...)
}

func (h CommandHandler) subsScreen(ctx context.Context, rt routing.Service, admin bool) screen {
	subs, err := rt.Subscriptions(ctx)
	if err != nil {
		return newScreen("📰 Подписки\n\n⚠ "+short(err.Error(), 300), navRow("main"))
	}
	n := rt.PendingCount(ctx)
	lines := []string{fmt.Sprintf("📰 Подписки (%d)", len(subs.URLs)), ""}
	if len(subs.URLs) == 0 {
		lines = append(lines, "Подписок нет.")
		if admin {
			lines = append(lines, "Нажмите «Добавить» и пришлите ссылку.")
		}
	}
	var dels []tgbotapi.InlineKeyboardButton
	for i, u := range subs.URLs {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, routing.MaskURL(u)))
		dels = append(dels, btn(fmt.Sprintf("🗑 %d", i+1), fmt.Sprintf("cmd:/sub_del %d", i+1)))
	}
	if subs.Interval != "" {
		lines = append(lines, "", "🔁 Автообновление: каждые "+intervalText(subs.Interval))
	}
	lines = append(lines, "Ссылки показаны не полностью: в них секретный токен.")
	text := strings.Join(lines, "\n") + pendingLine(n)

	var rows []buttonRow
	if admin {
		rows = append(rows, buttonRow{btn("➕ Добавить", "input:sub_add"), btn("🔄 Обновить сейчас", "cmd:/subscription_refresh")})
		for i := 0; i < len(dels); i += 4 {
			end := i + 4
			if end > len(dels) {
				end = len(dels)
			}
			rows = append(rows, buttonRow(dels[i:end]))
		}
		var iv buttonRow
		for _, v := range []string{"1h", "6h", "12h", "24h"} {
			label := intervalText(v)
			if v == subs.Interval {
				label = "✓ " + label
			}
			iv = append(iv, btn(label, "cmd:/sub_interval "+v))
		}
		rows = append(rows, iv)
		rows = append(rows, pendingRows(n, admin)...)
	}
	rows = append(rows, navRow("main"))
	return newScreen(text, rows...)
}

func intervalText(v string) string {
	if len(v) < 2 {
		return v
	}
	num, unit := v[:len(v)-1], v[len(v)-1]
	switch unit {
	case 'm':
		return num + " мин"
	case 'h':
		return num + " ч"
	case 'd':
		return num + " дн"
	}
	return v
}

func (h CommandHandler) listsScreen(ctx context.Context, rt routing.Service, admin bool, page int) screen {
	secs, err := rt.ListRoutes(ctx)
	if err != nil {
		return newScreen("🗂 Списки\n\n⚠ "+short(err.Error(), 300), navRow("main"))
	}
	flat := routing.FlattenLists(secs)
	if len(flat) == 0 {
		return newScreen("🗂 Списки\n\nСписков пока нет.", navRow("main"))
	}
	pages := (len(flat) + listsPerPage - 1) / listsPerPage
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	n := rt.PendingCount(ctx)
	head := "🗂 Списки → каналы"
	if pages > 1 {
		head += fmt.Sprintf(" · стр. %d из %d", page+1, pages)
	}
	lines := []string{head, ""}
	var rows []buttonRow
	start, end := page*listsPerPage, (page+1)*listsPerPage
	if end > len(flat) {
		end = len(flat)
	}
	for i := start; i < end; i++ {
		f := flat[i]
		target := routing.ChannelLabel(f.Section, f.List.Channel)
		lines = append(lines, "• "+routing.ListTitle(f.List)+" → "+target)
		label := short(routing.ListTitle(f.List), 18) + " → " + short(target, 14)
		rows = append(rows, buttonRow{btn(label, fmt.Sprintf("nav:lst:%d", i))})
	}
	if !admin {
		rows = nil
	} else {
		lines = append(lines, "", "Нажмите на список, чтобы выбрать для него канал.")
	}
	if pages > 1 {
		var pr buttonRow
		if page > 0 {
			pr = append(pr, btn("◀ Назад", fmt.Sprintf("nav:lists:%d", page-1)))
		}
		if page < pages-1 {
			pr = append(pr, btn("Вперёд ▶", fmt.Sprintf("nav:lists:%d", page+1)))
		}
		rows = append(rows, pr)
	}
	rows = append(rows, pendingRows(n, admin)...)
	rows = append(rows, navRow("main"))
	return newScreen(strings.Join(lines, "\n")+pendingLine(n), rows...)
}

func (h CommandHandler) listScreen(ctx context.Context, rt routing.Service, admin bool, idx int) screen {
	secs, err := rt.ListRoutes(ctx)
	if err != nil {
		return newScreen("🗂 Список\n\n⚠ "+short(err.Error(), 300), navRow("lists"))
	}
	flat := routing.FlattenLists(secs)
	if idx < 0 || idx >= len(flat) {
		return newScreen("Список не найден, возможно он изменился.", buttonRow{btn("🗂 К спискам", "nav:lists")})
	}
	f := flat[idx]
	back := fmt.Sprintf("lists:%d", idx/listsPerPage)
	lines := []string{
		"🗂 " + routing.ListTitle(f.List),
		"Секция: " + f.Section.Name,
		"Сейчас: " + routing.ChannelLabel(f.Section, f.List.Channel),
	}
	if !admin {
		return newScreen(strings.Join(lines, "\n"), navRow(back))
	}
	lines = append(lines, "", "Выберите канал:")
	cur := f.List.Channel
	if cur == "" {
		cur = listroutes.ChannelAuto
	}
	mark := func(id, label string) string {
		if cur == id {
			return "✓ " + label
		}
		return label
	}
	rows := []buttonRow{
		{btn(mark(listroutes.ChannelAuto, "🤖 Пул"), fmt.Sprintf("cmd:/rt %d auto", idx)), btn(mark(listroutes.ChannelBalance, "⚖️ Баланс"), fmt.Sprintf("cmd:/rt %d balance", idx))},
		{btn(mark(listroutes.ChannelDirect, "➡️ Напрямую"), fmt.Sprintf("cmd:/rt %d direct", idx)), btn(mark(listroutes.ChannelBlock, "⛔ Блок"), fmt.Sprintf("cmd:/rt %d block", idx))},
	}
	var chs []tgbotapi.InlineKeyboardButton
	for i, c := range f.Section.Channels {
		icon := "⚪"
		if c.Up != nil {
			icon = "❌"
			if *c.Up {
				icon = "✅"
			}
		}
		chs = append(chs, btn(mark(c.ID, icon+" "+short(c.Name, 20)), fmt.Sprintf("cmd:/rt %d %d", idx, i+1)))
	}
	rows = append(rows, pairs(chs)...)
	rows = append(rows, navRow(back))
	return newScreen(strings.Join(lines, "\n"), rows...)
}

func (h CommandHandler) settingsScreen(ctx context.Context, rt routing.Service, admin bool) screen {
	get := func(key string) string {
		v, err := rt.GetRouterParam(ctx, key)
		if err != nil || strings.TrimSpace(v) == "" {
			return ""
		}
		return strings.TrimSpace(v)
	}
	policy := get(rt.MainSectionKey("failover_policy"))
	interval := get(rt.MainSectionKey("urltest_check_interval"))
	tolerance := get(rt.MainSectionKey("urltest_tolerance"))
	idle := get(rt.MainSectionKey("urltest_idle_timeout"))
	interrupt := isOn(get(rt.MainSectionKey("urltest_interrupt_exist_connections")))
	quicOff := isOn(get(rt.SettingsKey("disable_quic")))
	dash := func(v, suffix string) string {
		if v == "" {
			return "не задано"
		}
		return v + suffix
	}
	n := rt.PendingCount(ctx)
	lines := []string{
		"⚙️ Настройки · секция " + rt.MainSection(),
		"",
		"🧭 Политика: " + dash(policyLabel(policy), ""),
		"⏱ Проверка каждые: " + dash(interval, ""),
		"📏 Допуск: " + dash(tolerance, " мс"),
		"💤 Idle timeout: " + dash(idle, ""),
		"✂️ Рвать соединения при смене: " + onOff(interrupt),
		"🌐 QUIC: " + map[bool]string{true: "выключен", false: "включён"}[quicOff],
	}
	text := strings.Join(lines, "\n") + pendingLine(n)
	if !admin {
		return newScreen(text, navRow("main"))
	}
	pol := func(p, label string) tgbotapi.InlineKeyboardButton {
		if p == policy {
			label = "✓ " + label
		}
		return btn(label, "cmd:/set_policy "+p)
	}
	numRow := func(cur string, vals []string, cmd, unit string) buttonRow {
		var r buttonRow
		for _, v := range vals {
			label := v + unit
			if cur == v || cur == v+"s" {
				label = "✓ " + label
			}
			r = append(r, btn(label, cmd+" "+v))
		}
		return r
	}
	rows := []buttonRow{
		{pol("outage-only", "Только при падении")},
		{pol("prefer-primary", "Основной в приоритете")},
		{pol("fastest", "Самый быстрый")},
		append(buttonRow{btn("⏱", "nav:settings")}, numRow(interval, []string{"15", "30", "60", "120"}, "cmd:/set_urltest_interval", " с")...),
		append(buttonRow{btn("📏", "nav:settings")}, numRow(tolerance, []string{"30", "50", "100", "200"}, "cmd:/set_urltest_tolerance", "")...),
		{quicButton(quicOff), interruptButton(interrupt)},
	}
	rows = append(rows, pendingRows(n, admin)...)
	rows = append(rows, buttonRow{btn("🔧 Расширенные", "nav:params"), btn("🧩 UCI", "nav:uci")}, navRow("main"))
	return newScreen(text, rows...)
}

func isOn(v string) bool {
	switch strings.ToLower(v) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func onOff(b bool) string {
	if b {
		return "да"
	}
	return "нет"
}

func (h CommandHandler) systemScreen(ctx context.Context, rt routing.Service, admin bool) screen {
	text := "🛠 Система\n\nСтатус, логи, события и обслуживание."
	rows := []buttonRow{
		{btn("📊 Статус", "cmd:/status"), btn("📜 Логи", "cmd:/logs 80")},
		{btn("🕘 История переключений", "cmd:/history")},
		{btn("👥 Клиенты", "cmd:/clients"), btn("🛡 Мониторинг", "nav:watch")},
	}
	if admin {
		rows = append(rows,
			buttonRow{btn("🔄 Обновить списки", "cmd:/list_update"), btn("🔁 Перезапуск", "cmd:/routing_restart")},
			buttonRow{btn("🤖 Конфиг бота", "nav:config")},
		)
	}
	rows = append(rows, navRow("main"))
	return newScreen(text, rows...)
}

// commandNav maps slash commands to the screens that replace their text answers.
var commandNav = map[string]string{
	"/start":    "main",
	"/menu":     "main",
	"/panel":    "main",
	"/channels": "channels",
	"/subs":     "subs",
	"/lists":    "lists",
	"/settings": "settings",
	"/system":   "system",
	"/manage":   "router",
	"/watch":    "watch",
}

// navAfter names the screen to show once cmd has run, so a button press ends
// on the section it came from instead of a bare confirmation.
func navAfter(cmd string) (string, bool) {
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return "", false
	}
	switch f[0] {
	case "/switch":
		return "channels", true
	case "/sub_add", "/sub_del", "/sub_interval", "/subscription_refresh":
		return "subs", true
	case "/rt":
		if len(f) >= 2 {
			if i, err := strconv.Atoi(f[1]); err == nil && i >= 0 {
				return fmt.Sprintf("lists:%d", i/listsPerPage), true
			}
		}
		return "lists", true
	case "/set_policy", "/set_quic", "/set_urltest_interval", "/set_urltest_tolerance",
		"/set_urltest_idle_timeout", "/set_interrupt_existing":
		return "settings", true
	case "/param_apply", "/param_rollback", "/use":
		return "main", true
	case "/list_update", "/routing_restart":
		return "system", true
	case "/repair", "/mute", "/unmute":
		return "watch", true
	}
	return routerNavAfter(f)
}

// actionNote is the one-line result shown above the screen after cmd.
func actionNote(cmd, resp string) string {
	f := strings.Fields(cmd)
	switch f[0] {
	case "/switch":
		return "✔ Канал переключён"
	case "/param_apply":
		return "✔ Изменения применены"
	case "/param_rollback":
		return "↩️ Изменения откачены"
	case "/sub_add":
		return "✔ Подписка добавлена, примените изменения"
	case "/sub_del":
		return "✔ Подписка удалена, примените изменения"
	case "/subscription_refresh":
		return "✔ Подписки обновлены"
	case "/rt", "/sub_interval", "/set_policy", "/set_quic", "/set_urltest_interval",
		"/set_urltest_tolerance", "/set_urltest_idle_timeout", "/set_interrupt_existing":
		return "✔ Сохранено, примените изменения"
	case "/use":
		return ""
	case "/repair":
		return "🔧 Сервис перезапущен"
	case "/mute":
		return "🔕 " + firstLine(resp)
	case "/unmute":
		return "🔔 Уведомления включены"
	}
	return "✔ " + firstLine(resp)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// confirmText explains what a confirmable command is about to do.
func (h CommandHandler) confirmText(ctx context.Context, userID int64, cmd string) string {
	f := strings.Fields(cmd)
	switch f[0] {
	case "/param_apply":
		return "Применить все ожидающие изменения? Движок перечитает конфигурацию."
	case "/param_rollback":
		return "Откатить все ожидающие изменения?"
	case "/routing_restart", "/repair":
		return "Перезапустить сервис? Активные соединения оборвутся."
	case "/sub_del":
		if len(f) == 2 {
			if n, err := strconv.Atoi(f[1]); err == nil {
				if rt, err := h.routingFor(userID); err == nil {
					if u, err := rt.SubscriptionAt(ctx, n); err == nil {
						return "Удалить подписку?\n" + routing.MaskURL(u)
					}
				}
			}
		}
	}
	return "Подтвердите действие: " + cmd
}

func quicButton(quicOff bool) tgbotapi.InlineKeyboardButton {
	if quicOff {
		return btn("QUIC: включить", "cmd:/set_quic on")
	}
	return btn("QUIC: выключить", "cmd:/set_quic off")
}

func interruptButton(on bool) tgbotapi.InlineKeyboardButton {
	if on {
		return btn("Рвать соединения: выкл", "cmd:/set_interrupt_existing off")
	}
	return btn("Рвать соединения: вкл", "cmd:/set_interrupt_existing on")
}

// watchLine is the one-line monitoring summary for the main screen.
func (h CommandHandler) watchLine() string {
	if h.wd == nil {
		return ""
	}
	var bad []string
	for _, st := range h.wd.Snapshot() {
		if !st.Healthy {
			bad = append(bad, st.Name)
		}
	}
	if len(bad) > 0 {
		return "🛡 Мониторинг: проблема на " + strings.Join(bad, ", ")
	}
	return "🛡 Мониторинг: всё в порядке"
}

func ago(t time.Time) string {
	if t.IsZero() {
		return "ещё не проверялось"
	}
	d := time.Since(t)
	switch {
	case d < 5*time.Second:
		return "только что"
	case d < time.Minute:
		return fmt.Sprintf("%d с назад", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d мин назад", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d ч назад", int(d.Hours()))
	}
}

func spanText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d с", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d мин", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d ч %d мин", int(d.Hours()), int(d.Minutes())%60)
	}
}

func (h CommandHandler) watchScreen(ctx context.Context, userID int64, admin, checkNow bool) screen {
	if h.wd == nil {
		return newScreen("🛡 Мониторинг\n\nВыключен в конфиге бота (watchdog_enabled).", navRow("main"))
	}
	if checkNow {
		if inst, err := h.mgr.InstanceFor(userID); err == nil {
			h.wd.CheckNow(ctx, inst.ID)
		}
	}
	cfg := h.wd.Config()
	repairs := "выкл"
	if cfg.AutoRepair {
		repairs = "вкл"
	}
	lines := []string{
		"🛡 Мониторинг",
		fmt.Sprintf("Проверка каждые %d с · авточинка: %s", int(cfg.Interval.Seconds()), repairs),
	}
	muted := h.wd.MutedUntil()
	if muted.IsZero() {
		lines = append(lines, "🔔 Уведомления включены")
	} else {
		lines = append(lines, "🔕 Уведомления выключены до "+muted.Local().Format("15:04"))
	}
	lines = append(lines, "")
	for _, st := range h.wd.Snapshot() {
		if st.Healthy {
			lines = append(lines, "✅ "+st.Name+": всё в порядке · проверено "+ago(st.Checked))
		} else {
			lines = append(lines, "⚠ "+st.Name+": проблема · проверено "+ago(st.Checked))
			for _, p := range st.Problems {
				lines = append(lines, "   • "+p)
			}
			if !st.Since.IsZero() {
				lines = append(lines, "   длится "+spanText(time.Since(st.Since))+fmt.Sprintf(", починок: %d из %d", st.Repairs, cfg.MaxRepairs))
			}
			if st.LastAction != "" {
				lines = append(lines, "   последнее: "+st.LastAction)
			}
		}
		if inc := st.LastIncident; inc != nil {
			how := "само"
			if len(inc.Actions) > 0 {
				how = strings.Join(inc.Actions, ", ")
			}
			lines = append(lines, fmt.Sprintf("   последний сбой: %s–%s (%s), %s",
				inc.From.Local().Format("15:04"), inc.To.Local().Format("15:04"), strings.Join(inc.Problems, "; "), how))
		}
	}
	lines = append(lines, "", "Если проблема не уходит сама, бот перезапускает сервис, а если не помогло, пишет вам.")
	rows := []buttonRow{{btn("🔍 Проверить сейчас", "nav:watch_check")}}
	if admin {
		rows = append(rows, buttonRow{btn("🔧 Починить сейчас", "cmd:/repair")})
		if muted.IsZero() {
			rows = append(rows, buttonRow{btn("🔕 Заглушить на 1 ч", "cmd:/mute 60")})
		} else {
			rows = append(rows, buttonRow{btn("🔔 Включить уведомления", "cmd:/unmute")})
		}
	}
	rows = append(rows, navRow("main"))
	return newScreen(strings.Join(lines, "\n"), rows...)
}
