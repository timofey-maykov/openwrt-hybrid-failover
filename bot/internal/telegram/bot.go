package telegram

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/audit"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/security"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

type Handler interface {
	Handle(ctx context.Context, userID int64, text string) (string, error)
}

type Bot struct {
	api            *tgbotapi.BotAPI
	auth           security.Authorizer
	audit          *audit.Logger
	h              Handler
	log            *slog.Logger
	confirmMu      sync.Mutex
	pendingConfirm map[int64]pendingConfirm
	inputMu        sync.Mutex
	pendingInput   map[int64]string
	identity       string
	adminIDs       []int64
}

// SetIdentity names the router this bot instance runs on; it is shown in
// failover notices and conflict warnings. adminIDs receive those notices.
func (b *Bot) SetIdentity(name string, adminIDs []int64) {
	b.identity = name
	b.adminIDs = adminIDs
}

type pendingConfirm struct {
	cmd       string
	id        string // set for token confirmations (long, typed or router commands)
	router    string // router selected when it was asked for
	expiresAt time.Time
}

func New(api *tgbotapi.BotAPI, auth security.Authorizer, auditLogger *audit.Logger, h Handler, log *slog.Logger) *Bot {
	return &Bot{
		api:            api,
		auth:           auth,
		audit:          auditLogger,
		h:              h,
		log:            log,
		pendingConfirm: map[int64]pendingConfirm{},
		pendingInput:   map[int64]string{},
	}
}

// NotifyAdmins sends text to every admin chat (used by the watchdog).
func (b *Bot) NotifyAdmins(text string) { b.notifyAdmins(text) }

func (b *Bot) Run(ctx context.Context) error {
	b.registerCommands()
	return b.poll(ctx, b.dispatchUpdate)
}

// registerCommands fills the "/" menu in Telegram with the sections.
func (b *Bot) registerCommands() {
	cmds := tgbotapi.NewSetMyCommands(
		tgbotapi.BotCommand{Command: "menu", Description: "Главное меню"},
		tgbotapi.BotCommand{Command: "channels", Description: "Каналы и переключение"},
		tgbotapi.BotCommand{Command: "subs", Description: "Подписки"},
		tgbotapi.BotCommand{Command: "lists", Description: "Списки и их каналы"},
		tgbotapi.BotCommand{Command: "settings", Description: "Настройки failover"},
		tgbotapi.BotCommand{Command: "system", Description: "Статус, логи, обслуживание"},
		tgbotapi.BotCommand{Command: "manage", Description: "Управление роутером"},
		tgbotapi.BotCommand{Command: "watch", Description: "Мониторинг и авточинка"},
		tgbotapi.BotCommand{Command: "health", Description: "Проверить каналы сейчас"},
		tgbotapi.BotCommand{Command: "help", Description: "Список всех команд"},
	)
	if _, err := b.api.Request(cmds); err != nil {
		b.log.Warn("set bot commands failed", "err", err)
	}
}

func (b *Bot) dispatchUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.CallbackQuery != nil && update.CallbackQuery.From != nil && update.CallbackQuery.Message != nil {
		b.handleCallback(ctx, update.CallbackQuery)
		return
	}
	if update.Message == nil || update.Message.From == nil {
		return
	}
	if strings.TrimSpace(update.Message.Text) == "" {
		return
	}
	b.handleMessage(ctx, update.Message.Chat.ID, update.Message.From.ID, update.Message.Text)
}

func (b *Bot) handleMessage(ctx context.Context, chatID int64, userID int64, text string) {
	action := strings.Fields(text)
	actionName := "unknown"
	if len(action) > 0 {
		actionName = action[0]
	}

	if !b.auth.Allowed(userID, text) {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "denied"})
		b.reply(chatID, "Доступ запрещен: нет прав для этой команды.")
		return
	}
	if b.auth.IsViewer(userID) && !b.auth.IsAdmin(userID) {
		if kind, ok := b.getPendingInput(userID); ok && kind != "" {
			b.reply(chatID, "Режим только чтение: изменение конфигурации запрещено.")
			return
		}
	}

	if strings.TrimSpace(text) == "/cancel" {
		b.clearInput(userID)
		b.reply(chatID, "Ввод отменен.")
		return
	}
	// A new slash command abandons the prompt instead of being taken as its value.
	if strings.HasPrefix(strings.TrimSpace(text), "/") {
		b.clearInput(userID)
	}
	if kind, ok := b.getPendingInput(userID); ok {
		cmd, err := inputToCommand(kind, text)
		if err != nil {
			b.reply(chatID, "Ошибка ввода: "+err.Error()+"\nПовторите ввод или /cancel")
			return
		}
		b.clearInput(userID)
		if needsConfirm(strings.Fields(cmd)) {
			b.askConfirm(chatID, userID, cmd)
			return
		}
		resp, err := b.h.Handle(ctx, userID, cmd)
		if err != nil {
			_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "error", Details: err.Error()})
			b.replyResult(ctx, chatID, userID, cmd, "⚠ Ошибка: "+err.Error())
			return
		}
		_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "ok"})
		b.replyResult(ctx, chatID, userID, cmd, actionNote(cmd, resp))
		return
	}

	if nav, ok := commandNav[actionName]; ok && len(action) == 1 {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "ok"})
		b.sendScreen(ctx, chatID, userID, nav, "")
		return
	}

	if needsConfirm(strings.Fields(text)) {
		b.askConfirm(chatID, userID, strings.TrimSpace(text))
		return
	}

	resp, err := b.h.Handle(ctx, userID, text)
	if err != nil {
		b.log.Error("command failed", "user_id", userID, "cmd", actionName, "err", err)
		_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "error", Details: err.Error()})
		b.reply(chatID, "Ошибка: "+err.Error())
		return
	}

	_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "ok"})
	if actionName == "/param_menu" {
		b.replyWithParamMenu(chatID, resp)
		return
	}
	b.reply(chatID, resp)
}

func (b *Bot) reply(chatID int64, text string) {
	b.sendChunks(chatID, text, nil)
}

// sendChunks sends text split to Telegram's message limit; the keyboard, if
// any, goes on the last part.
func (b *Bot) sendChunks(chatID int64, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	parts := splitMessage(text)
	for i, part := range parts {
		msg := tgbotapi.NewMessage(chatID, part)
		if keyboard != nil && i == len(parts)-1 {
			msg.ReplyMarkup = *keyboard
		}
		if _, err := b.api.Send(msg); err != nil {
			b.log.Error("send failed", "chat_id", chatID, "err", err)
		}
	}
}

func (b *Bot) replyWithParamMenu(chatID int64, text string) {
	keyboard := paramMenuKeyboard()
	b.sendChunks(chatID, text, &keyboard)
}

// renderScreen builds nav for the user; ok is false for panels that have no
// screen (the caller then falls back to the classic menus).
func (b *Bot) renderScreen(ctx context.Context, userID int64, nav string) (screen, bool) {
	ch, ok := b.h.(CommandHandler)
	if !ok {
		return screen{}, false
	}
	return ch.render(ctx, userID, b.auth.IsAdmin(userID), nav)
}

// sendScreen posts a new message with the screen, note on top.
func (b *Bot) sendScreen(ctx context.Context, chatID, userID int64, nav, note string) {
	sc, ok := b.renderScreen(ctx, userID, nav)
	if !ok {
		sc = screen{text: b.panelIntro(), kb: paramMenuKeyboard()}
	}
	text := sc.text
	if note != "" {
		text = note + "\n\n" + text
	}
	b.sendChunks(chatID, text, &sc.kb)
}

// replyResult answers a typed command: the screen it belongs to if it has one.
func (b *Bot) replyResult(ctx context.Context, chatID, userID int64, cmd, note string) {
	if nav, ok := navAfter(cmd); ok {
		b.sendScreen(ctx, chatID, userID, nav, note)
		return
	}
	b.reply(chatID, note)
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID
	actionName := "callback"

	isAdmin := b.auth.IsAdmin(userID)
	if !isAdmin && !b.auth.IsViewer(userID) {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "denied"})
		b.answerCallback(cb.ID, "Доступ запрещен")
		return
	}
	deny := func(action string) {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: action, Result: "denied"})
		b.answerCallback(cb.ID, "Только чтение: действие недоступно")
	}

	// Navigation is read-only and open to viewers too.
	if nav, ok := callbackToNav(cb.Data); ok {
		b.answerCallback(cb.ID, "")
		b.editNavPanel(ctx, chatID, cb.Message.MessageID, nav, userID)
		return
	}
	if token, ok := callbackToConfirmToken(cb.Data); ok {
		if !isAdmin {
			deny("confirm")
			return
		}
		cmd, ok := b.takeConfirmToken(userID, token)
		if !ok {
			b.answerCallback(cb.ID, "Подтверждение устарело")
			b.editOrReply(chatID, cb.Message.MessageID, "Подтверждение устарело или роутер сменился, повторите действие.")
			return
		}
		b.runCommandFromCallback(ctx, cb.ID, chatID, cb.Message.MessageID, userID, cmd)
		return
	}
	if confirmCmd, ok := callbackToConfirm(cb.Data); ok {
		if !isAdmin {
			deny(confirmCmd)
			return
		}
		if !b.isConfirmAllowed(userID, confirmCmd) {
			b.answerCallback(cb.ID, "Подтверждение устарело")
			b.editOrReply(chatID, cb.Message.MessageID, "Подтверждение устарело, повторите действие.")
			return
		}
		b.clearConfirm(userID)
		b.runCommandFromCallback(ctx, cb.ID, chatID, cb.Message.MessageID, userID, confirmCmd)
		return
	}
	if cb.Data == "input_cancel" {
		b.clearInput(userID)
		b.answerCallback(cb.ID, "")
		b.editNavPanel(ctx, chatID, cb.Message.MessageID, "main", userID)
		return
	}
	if inputKind, ok := callbackToInput(cb.Data); ok {
		if !isAdmin {
			deny("input:" + inputKind)
			return
		}
		b.setPendingInput(userID, inputKind)
		b.answerCallback(cb.ID, "")
		k := inputCancelKeyboard()
		b.editOrReplyWithKeyboard(chatID, cb.Message.MessageID, b.promptForInput(inputKind), &k)
		return
	}
	cmd, ok := callbackToCommand(cb.Data)
	if !ok {
		b.answerCallback(cb.ID, "Неизвестная кнопка")
		return
	}
	if !b.auth.Allowed(userID, cmd) {
		deny(cmd)
		return
	}
	if needsConfirm(strings.Fields(cmd)) {
		if !b.confirmable(cb.ID, userID, cmd) {
			return
		}
		b.answerCallback(cb.ID, "Требуется подтверждение")
		b.askConfirm(chatID, userID, cmd)
		return
	}
	if b.requiresConfirmation(cmd) {
		b.setConfirm(userID, cmd)
		b.answerCallback(cb.ID, "Требуется подтверждение")
		b.replyWithConfirm(chatID, b.confirmPrompt(ctx, userID, cmd), cmd)
		return
	}
	b.runCommandFromCallback(ctx, cb.ID, chatID, cb.Message.MessageID, userID, cmd)
}

func (b *Bot) runCommandFromCallback(ctx context.Context, callbackID string, chatID int64, messageID int, userID int64, cmd string) {
	// Answer right away: Telegram keeps the button spinning until it gets an
	// answer, and restart or list-update can take much longer than that.
	b.answerCallback(callbackID, "Выполняю…")
	if cmd == "/backup" {
		b.sendBackup(ctx, chatID, userID)
		return
	}
	resp, err := b.h.Handle(ctx, userID, cmd)
	nav, hasNav := navAfter(cmd)
	if err != nil {
		b.log.Error("callback command failed", "user_id", userID, "cmd", cmd, "err", err)
		_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "error", Details: err.Error()})
		if hasNav {
			b.showScreen(ctx, chatID, messageID, userID, nav, "⚠ Ошибка: "+err.Error())
			return
		}
		b.editOrReplyWithKeyboard(chatID, messageID, "Ошибка: "+err.Error(), keyboardForCmd(cmd))
		return
	}

	_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "ok"})
	if hasNav {
		b.showScreen(ctx, chatID, messageID, userID, nav, actionNote(cmd, resp))
		return
	}
	b.editOrReplyWithKeyboard(chatID, messageID, resp, keyboardForCmd(cmd))
}

// showScreen edits the message into the screen for nav, note on top.
func (b *Bot) showScreen(ctx context.Context, chatID int64, messageID int, userID int64, nav, note string) {
	sc, ok := b.renderScreen(ctx, userID, nav)
	if !ok {
		b.editOrReplyWithKeyboard(chatID, messageID, note, keyboardForCmd(""))
		return
	}
	text := sc.text
	if note != "" {
		text = note + "\n\n" + text
	}
	b.editOrReplyWithKeyboard(chatID, messageID, text, &sc.kb)
}

func (b *Bot) confirmPrompt(ctx context.Context, userID int64, cmd string) string {
	if ch, ok := b.h.(CommandHandler); ok {
		return ch.confirmText(ctx, userID, cmd)
	}
	return "Подтвердите действие: " + cmd
}

func (b *Bot) answerCallback(callbackID, text string) {
	c := tgbotapi.NewCallback(callbackID, text)
	_, _ = b.api.Request(c)
}

func (b *Bot) editNavPanel(ctx context.Context, chatID int64, messageID int, nav string, userID int64) {
	if nav == "watch_check" {
		b.editOrReplyWithKeyboard(chatID, messageID, "⏳ Проверяю роутер…", nil)
	}
	if nav == "channels_probe" {
		// Fresh probes take a while; say so instead of leaving a frozen button.
		b.editOrReplyWithKeyboard(chatID, messageID, "⏳ Проверяю каналы…", nil)
	}
	if sc, ok := b.renderScreen(ctx, userID, nav); ok {
		b.editOrReplyWithKeyboard(chatID, messageID, sc.text, &sc.kb)
		return
	}
	var text string
	var keyboard tgbotapi.InlineKeyboardMarkup
	switch nav {
	case "params":
		if ch, ok := b.h.(CommandHandler); ok {
			text = ch.paramMenuText(userID)
		} else {
			text = paramMenuText()
		}
		keyboard = paramMenuKeyboard()
	case "config":
		text = "🤖 Конфиг бота"
		keyboard = configKeyboard()
	case "uci":
		if ch, ok := b.h.(CommandHandler); ok {
			text = ch.uciMenuText(userID)
		} else {
			text = uciMenuText()
		}
		keyboard = uciKeyboard()
	default:
		text = b.panelIntro()
		keyboard = paramMenuKeyboard()
	}
	b.editOrReplyWithKeyboard(chatID, messageID, text, &keyboard)
}

func (b *Bot) replyWithConfirm(chatID int64, text, cmd string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = confirmKeyboard(cmd)
	_, _ = b.api.Send(msg)
}

func (b *Bot) editOrReply(chatID int64, messageID int, text string) {
	b.editOrReplyWithKeyboard(chatID, messageID, text, nil)
}

func (b *Bot) editOrReplyWithKeyboard(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if parts := splitMessage(text); len(parts) > 1 {
		// Too long for one message: put the head in place, the rest below.
		b.editOrReplyWithKeyboard(chatID, messageID, parts[0], nil)
		b.sendChunks(chatID, strings.Join(parts[1:], "\n"), keyboard)
		return
	}
	if messageID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		if keyboard != nil {
			edit.ReplyMarkup = keyboard
		}
		_, err := b.api.Send(edit)
		if err == nil || strings.Contains(err.Error(), "message is not modified") {
			return
		}
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if keyboard != nil {
		msg.ReplyMarkup = *keyboard
	}
	_, _ = b.api.Send(msg)
}

func (b *Bot) panelIntro() string {
	if ch, ok := b.h.(CommandHandler); ok {
		return mainPanelText(ch.mgr)
	}
	return mainPanelText(nil)
}

// keyboardForCmd is the keyboard under a plain text answer, for commands that
// have no screen of their own.
func keyboardForCmd(cmd string) *tgbotapi.InlineKeyboardMarkup {
	switch {
	case strings.HasPrefix(cmd, "/set_"), strings.HasPrefix(cmd, "/param_"), cmd == "/params":
		k := paramMenuKeyboard()
		return &k
	case strings.HasPrefix(cmd, "/uci_"):
		k := uciKeyboard()
		return &k
	case strings.HasPrefix(cmd, "/config_"):
		k := configKeyboard()
		return &k
	case isSysCommand(cmd):
		k := tgbotapi.NewInlineKeyboardMarkup(navRow("router"))
		return &k
	default:
		k := tgbotapi.NewInlineKeyboardMarkup(navRow("system"))
		return &k
	}
}

func (b *Bot) requiresConfirmation(cmd string) bool {
	switch cmd {
	case "/param_apply", "/param_rollback", "/failover_apply", "/routing_restart", "/config_apply", "/config_rollback":
		return true
	}
	return cmd == "/repair" || strings.HasPrefix(cmd, "/sub_del ")
}

func (b *Bot) setConfirm(userID int64, cmd string) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	b.pendingConfirm[userID] = pendingConfirm{
		cmd:       cmd,
		expiresAt: time.Now().Add(30 * time.Second),
	}
}

func (b *Bot) isConfirmAllowed(userID int64, cmd string) bool {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	state, ok := b.pendingConfirm[userID]
	if !ok {
		return false
	}
	if time.Now().After(state.expiresAt) {
		delete(b.pendingConfirm, userID)
		return false
	}
	return state.cmd == cmd
}

func (b *Bot) clearConfirm(userID int64) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	delete(b.pendingConfirm, userID)
}

func (b *Bot) setPendingInput(userID int64, kind string) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	b.pendingInput[userID] = kind
}

func (b *Bot) getPendingInput(userID int64) (string, bool) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	kind, ok := b.pendingInput[userID]
	return kind, ok
}

func (b *Bot) clearInput(userID int64) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	delete(b.pendingInput, userID)
}

func (b *Bot) uciExample(option string) string {
	if ch, ok := b.h.(interface{ UCISectionKey(string) string }); ok {
		return ch.UCISectionKey(option)
	}
	return paths.UCIPackage + "." + paths.DefaultMainSection + "." + option
}

func (b *Bot) promptForInput(kind string) string {
	switch kind {
	case "urltest_interval":
		return "Введите URLTest check_interval (например: 30 или 30s). Для отмены: /cancel"
	case "urltest_tolerance":
		return "Введите URLTest tolerance в миллисекундах (например: 100). Для отмены: /cancel"
	case "urltest_idle_timeout":
		return "Введите URLTest idle timeout в секундах (например: 60). Для отмены: /cancel"
	case "interrupt_existing":
		return "Введите interrupt existing: on или off. Для отмены: /cancel"
	case "sub_add":
		return "📰 Пришлите ссылку на подписку одним сообщением (https://…).\nОтмена: /cancel"
	case "portfwd_add":
		return "🔀 Введите: <tcp|udp|tcpudp> <порт> <ip> [порт_назначения] [имя]\nПример: tcp 2222 192.168.1.5 22 ssh\nОтмена: /cancel"
	case "ping":
		return "🏓 Введите адрес или имя хоста для ping.\nОтмена: /cancel"
	case "traceroute":
		return "🧭 Введите адрес или имя хоста для traceroute.\nОтмена: /cancel"
	case "uci_get":
		return "Введите ключ: hybrid-failover.section.option\nПример: " + b.uciExample("urltest_check_interval") + "\nОтмена: /cancel"
	case "uci_set":
		return "Введите: <ключ> <значение>\nПример: " + b.uciExample("urltest_check_interval") + " 45s\nОтмена: /cancel"
	case "uci_add_list":
		return "Введите: <ключ> <значение>\nПример: " + b.uciExample("failover_proxy_links") + " vless://...\nОтмена: /cancel"
	case "uci_del_list":
		return "Введите: <ключ> <значение>\nПример: " + b.uciExample("failover_proxy_links") + " vless://...\nОтмена: /cancel"
	case "uci_del":
		return "Введите ключ: hybrid-failover.section.option\nПример: " + b.uciExample("urltest_tolerance") + "\nОтмена: /cancel"
	default:
		return "Введите значение. Для отмены: /cancel"
	}
}

func inputToCommand(kind, value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("пустое значение")
	}
	switch kind {
	case "urltest_interval":
		return "/set_urltest_interval " + v, nil
	case "urltest_tolerance":
		return "/set_urltest_tolerance " + v, nil
	case "urltest_idle_timeout":
		return "/set_urltest_idle_timeout " + v, nil
	case "interrupt_existing":
		return "/set_interrupt_existing " + v, nil
	case "sub_add":
		if strings.ContainsAny(v, " \t\n") {
			return "", fmt.Errorf("нужна одна ссылка без пробелов")
		}
		return "/sub_add " + v, nil
	case "uci_get":
		return "/uci_get " + v, nil
	case "uci_set":
		parts := strings.Fields(v)
		if len(parts) < 2 {
			return "", fmt.Errorf("нужно указать ключ и значение")
		}
		return "/uci_set " + parts[0] + " " + strings.Join(parts[1:], " "), nil
	case "uci_add_list":
		parts := strings.Fields(v)
		if len(parts) < 2 {
			return "", fmt.Errorf("нужно указать ключ и значение")
		}
		return "/uci_add_list " + parts[0] + " " + strings.Join(parts[1:], " "), nil
	case "uci_del_list":
		parts := strings.Fields(v)
		if len(parts) < 2 {
			return "", fmt.Errorf("нужно указать ключ и значение")
		}
		return "/uci_del_list " + parts[0] + " " + strings.Join(parts[1:], " "), nil
	case "uci_del":
		return "/uci_del " + v, nil
	case "portfwd_add":
		return "/portfwd_add " + v, nil
	case "ping":
		return "/ping " + v, nil
	case "traceroute":
		return "/traceroute " + v, nil
	default:
		return "", fmt.Errorf("неизвестный тип ввода")
	}
}

func (b *Bot) canShell(userID int64) bool {
	if ch, ok := b.h.(CommandHandler); ok {
		return ch.ShellAllowed(userID) && b.auth.IsAdmin(userID)
	}
	return false
}

// confirmable checks, before a confirmation is offered, that the command can
// be run by this user at all, so the question is not asked for nothing.
func (b *Bot) confirmable(callbackID string, userID int64, cmd string) bool {
	if strings.HasPrefix(cmd, "/sh") && !b.canShell(userID) {
		b.answerCallback(callbackID, "/sh выключена")
		return false
	}
	return true
}

// askConfirm sends the confirmation question for a router command, naming the
// router when there are several.
func (b *Bot) askConfirm(chatID, userID int64, cmd string) {
	if strings.HasPrefix(cmd, "/sh") && !b.canShell(userID) {
		b.reply(chatID, "Ошибка: /sh выключена. Чтобы включить, добавьте свой ID в allow_shell_ids в /etc/hybrid-failover-bot.json и перезапустите бота.")
		return
	}
	id := b.setConfirmToken(userID, cmd)
	b.replyWithConfirmToken(chatID, b.routerPrefix(userID)+confirmPrompt(cmd), id)
}

func (b *Bot) setConfirmToken(userID int64, cmd string) string {
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	b.pendingConfirm[userID] = pendingConfirm{cmd: cmd, id: id, router: b.selectedRouter(userID), expiresAt: time.Now().Add(30 * time.Second)}
	return id
}

// takeConfirmToken returns the pending command for this user when the token
// matches, has not expired and the router has not been changed since.
func (b *Bot) takeConfirmToken(userID int64, id string) (string, bool) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	state, ok := b.pendingConfirm[userID]
	if !ok || state.id == "" || state.id != id || time.Now().After(state.expiresAt) {
		return "", false
	}
	// the router was changed with /use after the question was asked: do not
	// run the command on a router the user did not confirm it for
	if state.router != b.selectedRouter(userID) {
		delete(b.pendingConfirm, userID)
		return "", false
	}
	delete(b.pendingConfirm, userID)
	return state.cmd, true
}

func (b *Bot) replyWithConfirmToken(chatID int64, text, id string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = confirmTokenKeyboard(id)
	_, _ = b.api.Send(msg)
}

// selectedRouter is the router this user's commands go to right now.
func (b *Bot) selectedRouter(userID int64) string {
	if ch, ok := b.h.(CommandHandler); ok && ch.mgr != nil {
		return ch.mgr.SelectedID(userID)
	}
	return ""
}

// routerPrefix names the router in a confirmation when there are several.
func (b *Bot) routerPrefix(userID int64) string {
	if ch, ok := b.h.(CommandHandler); ok && ch.mgr != nil {
		return ch.mgr.Prefix(userID)
	}
	return ""
}

// sendBackup sends the configuration archive as a file.
func (b *Bot) sendBackup(ctx context.Context, chatID, userID int64) {
	ch, ok := b.h.(CommandHandler)
	if !ok {
		b.reply(chatID, "Ошибка: архив недоступен.")
		return
	}
	name, data, err := ch.Backup(ctx, userID)
	if err != nil {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: "/backup", Result: "error", Details: err.Error()})
		b.reply(chatID, "Ошибка: "+err.Error())
		return
	}
	doc := tgbotapi.NewDocument(chatID, tgbotapi.FileBytes{Name: name, Bytes: data})
	doc.Caption = "Архив настроек. В нём пароли и ключи, не пересылайте его."
	if _, err := b.api.Send(doc); err != nil {
		b.log.Error("send backup failed", "err", err)
		_ = b.audit.Write(audit.Event{UserID: userID, Action: "/backup", Result: "error", Details: err.Error()})
		b.reply(chatID, "Ошибка отправки архива в Telegram.")
		return
	}
	_ = b.audit.Write(audit.Event{UserID: userID, Action: "/backup", Result: "ok", Details: fmt.Sprintf("%d bytes", len(data))})
}
