package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"strings"
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
	audit          audit.Logger
	h              Handler
	log            *slog.Logger
	confirmMu      sync.Mutex
	pendingConfirm map[int64]pendingConfirm
	inputMu        sync.Mutex
	pendingInput   map[int64]string
}

type pendingConfirm struct {
	cmd       string
	expiresAt time.Time
}

func New(api *tgbotapi.BotAPI, auth security.Authorizer, auditLogger audit.Logger, h Handler, log *slog.Logger) Bot {
	return Bot{
		api:            api,
		auth:           auth,
		audit:          auditLogger,
		h:              h,
		log:            log,
		pendingConfirm: map[int64]pendingConfirm{},
		pendingInput:   map[int64]string{},
	}
}

func (b Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 50
	updates := b.api.GetUpdatesChan(u)
	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case update, ok := <-updates:
			if !ok {
				return fmt.Errorf("telegram updates channel closed")
			}
			if update.CallbackQuery != nil && update.CallbackQuery.From != nil && update.CallbackQuery.Message != nil {
				b.handleCallback(ctx, update.CallbackQuery)
				continue
			}
			if update.Message == nil || update.Message.From == nil {
				continue
			}
			b.handleMessage(ctx, update.Message.Chat.ID, update.Message.From.ID, update.Message.Text)
		}
	}
}

func (b Bot) handleMessage(ctx context.Context, chatID int64, userID int64, text string) {
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
	if kind, ok := b.getPendingInput(userID); ok {
		cmd, err := inputToCommand(kind, text)
		if err != nil {
			b.reply(chatID, "Ошибка ввода: "+err.Error()+"\nПовторите ввод или /cancel")
			return
		}
		b.clearInput(userID)
		resp, err := b.h.Handle(ctx, userID, cmd)
		if err != nil {
			b.reply(chatID, "Ошибка: "+err.Error())
			return
		}
		b.reply(chatID, resp)
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
	if actionName == "/panel" {
		b.replyWithMainPanel(chatID, resp)
		return
	}
	b.reply(chatID, resp)
}

func (b Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = b.api.Send(msg)
}

func (b Bot) replyWithParamMenu(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	keyboard := paramMenuKeyboard()
	msg.ReplyMarkup = keyboard
	_, _ = b.api.Send(msg)
}

func (b Bot) replyWithMainPanel(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	keyboard := mainPanelKeyboard()
	msg.ReplyMarkup = keyboard
	_, _ = b.api.Send(msg)
}

func (b Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	userID := cb.From.ID
	chatID := cb.Message.Chat.ID
	actionName := "callback"

	if !b.auth.IsAdmin(userID) {
		_ = b.audit.Write(audit.Event{UserID: userID, Action: actionName, Result: "denied"})
		b.answerCallback(cb.ID, "Доступ запрещен")
		return
	}

	if nav, ok := callbackToNav(cb.Data); ok {
		b.answerCallback(cb.ID, "")
		b.editNavPanel(chatID, cb.Message.MessageID, nav)
		return
	}
	if confirmCmd, ok := callbackToConfirm(cb.Data); ok {
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
		b.editOrReplyWithKeyboard(chatID, cb.Message.MessageID, "Ввод отменен.", failoverKeyboardPtr())
		return
	}
	if inputKind, ok := callbackToInput(cb.Data); ok {
		b.setPendingInput(userID, inputKind)
		b.answerCallback(cb.ID, "")
		k := inputCancelKeyboard()
		if strings.HasPrefix(inputKind, "uci_") {
			uk := inputCancelKeyboard()
			b.editOrReplyWithKeyboard(chatID, cb.Message.MessageID, b.promptForInput(inputKind), &uk)
		} else {
			b.editOrReplyWithKeyboard(chatID, cb.Message.MessageID, b.promptForInput(inputKind), &k)
		}
		return
	}
	cmd, ok := callbackToCommand(cb.Data)
	if !ok {
		b.answerCallback(cb.ID, "Неизвестная кнопка")
		return
	}
	if b.requiresConfirmation(cmd) {
		b.setConfirm(userID, cmd)
		b.answerCallback(cb.ID, "Требуется подтверждение")
		b.replyWithConfirm(chatID, "Подтвердите действие: "+cmd, cmd)
		return
	}
	b.runCommandFromCallback(ctx, cb.ID, chatID, cb.Message.MessageID, userID, cmd)
}

func (b Bot) runCommandFromCallback(ctx context.Context, callbackID string, chatID int64, messageID int, userID int64, cmd string) {
	resp, err := b.h.Handle(ctx, userID, cmd)
	if err != nil {
		b.log.Error("callback command failed", "user_id", userID, "cmd", cmd, "err", err)
		_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "error", Details: err.Error()})
		b.answerCallback(callbackID, "Ошибка")
		b.editOrReplyWithKeyboard(chatID, messageID, "Ошибка: "+err.Error(), keyboardForCmd(cmd))
		return
	}

	_ = b.audit.Write(audit.Event{UserID: userID, Action: cmd, Result: "ok"})
	b.answerCallback(callbackID, "")
	b.editOrReplyWithKeyboard(chatID, messageID, resp, keyboardForCmd(cmd))
}

func (b Bot) answerCallback(callbackID, text string) {
	c := tgbotapi.NewCallback(callbackID, text)
	_, _ = b.api.Request(c)
}

func (b Bot) editNavPanel(chatID int64, messageID int, nav string) {
	text := "Раздел: " + nav
	keyboard := mainPanelKeyboard()
	switch nav {
	case "main":
		text = mainPanelText()
		keyboard = mainPanelKeyboard()
	case "params":
		if ch, ok := b.h.(CommandHandler); ok {
			text = ch.paramMenuText()
		} else {
			text = paramMenuText()
		}
		keyboard = paramMenuKeyboard()
	case "service":
		text = "Раздел: Сервис"
		keyboard = serviceKeyboard()
	case "failover":
		text = "Раздел: Фейловер"
		keyboard = failoverKeyboard()
	case "config":
		text = "Раздел: Конфиг"
		keyboard = configKeyboard()
	case "uci":
		if ch, ok := b.h.(CommandHandler); ok {
			text = ch.uciMenuText()
		} else {
			text = uciMenuText()
		}
		keyboard = uciKeyboard()
	default:
		text = mainPanelText()
		keyboard = mainPanelKeyboard()
	}
	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	edit.ReplyMarkup = &keyboard
	if _, err := b.api.Send(edit); err != nil {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ReplyMarkup = keyboard
		_, _ = b.api.Send(msg)
	}
}

func (b Bot) replyWithConfirm(chatID int64, text, cmd string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = confirmKeyboard(cmd)
	_, _ = b.api.Send(msg)
}

func (b Bot) editOrReply(chatID int64, messageID int, text string) {
	b.editOrReplyWithKeyboard(chatID, messageID, text, nil)
}

func (b Bot) editOrReplyWithKeyboard(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) {
	if messageID > 0 {
		edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
		if keyboard != nil {
			edit.ReplyMarkup = keyboard
		}
		if _, err := b.api.Send(edit); err == nil {
			return
		}
	}
	msg := tgbotapi.NewMessage(chatID, text)
	if keyboard != nil {
		msg.ReplyMarkup = *keyboard
	}
	_, _ = b.api.Send(msg)
}

func keyboardForCmd(cmd string) *tgbotapi.InlineKeyboardMarkup {
	switch {
	case strings.HasPrefix(cmd, "/set_"), strings.HasPrefix(cmd, "/param_"), cmd == "/params":
		k := paramMenuKeyboard()
		return &k
	case strings.HasPrefix(cmd, "/uci_"):
		k := uciKeyboard()
		return &k
	case strings.HasPrefix(cmd, "/failover_"), strings.HasPrefix(cmd, "/switch"):
		k := failoverKeyboard()
		return &k
	case strings.HasPrefix(cmd, "/config_"):
		k := configKeyboard()
		return &k
	case cmd == "/status", cmd == "/routing_restart", strings.HasPrefix(cmd, "/logs"), cmd == "/channels":
		k := serviceKeyboard()
		return &k
	default:
		k := mainPanelKeyboard()
		return &k
	}
}

func (b Bot) requiresConfirmation(cmd string) bool {
	switch cmd {
	case "/param_apply", "/param_rollback", "/failover_apply", "/routing_restart", "/config_apply", "/config_rollback":
		return true
	default:
		return false
	}
}

func (b Bot) setConfirm(userID int64, cmd string) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	b.pendingConfirm[userID] = pendingConfirm{
		cmd:       cmd,
		expiresAt: time.Now().Add(30 * time.Second),
	}
}

func (b Bot) isConfirmAllowed(userID int64, cmd string) bool {
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

func (b Bot) clearConfirm(userID int64) {
	b.confirmMu.Lock()
	defer b.confirmMu.Unlock()
	delete(b.pendingConfirm, userID)
}

func (b Bot) setPendingInput(userID int64, kind string) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	b.pendingInput[userID] = kind
}

func (b Bot) getPendingInput(userID int64) (string, bool) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	kind, ok := b.pendingInput[userID]
	return kind, ok
}

func (b Bot) clearInput(userID int64) {
	b.inputMu.Lock()
	defer b.inputMu.Unlock()
	delete(b.pendingInput, userID)
}

func (b Bot) uciExample(option string) string {
	if ch, ok := b.h.(interface{ UCISectionKey(string) string }); ok {
		return ch.UCISectionKey(option)
	}
	return paths.UCIPackage + "." + paths.DefaultMainSection + "." + option
}

func (b Bot) promptForInput(kind string) string {
	switch kind {
	case "urltest_interval":
		return "Введите URLTest check_interval (например: 30 или 30s). Для отмены: /cancel"
	case "urltest_tolerance":
		return "Введите URLTest tolerance в миллисекундах (например: 100). Для отмены: /cancel"
	case "urltest_idle_timeout":
		return "Введите URLTest idle timeout в секундах (например: 60). Для отмены: /cancel"
	case "interrupt_existing":
		return "Введите interrupt existing: on или off. Для отмены: /cancel"
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

func promptForInput(kind string) string {
	return Bot{}.promptForInput(kind)
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
	default:
		return "", fmt.Errorf("неизвестный тип ввода")
	}
}

func failoverKeyboardPtr() *tgbotapi.InlineKeyboardMarkup {
	k := failoverKeyboard()
	return &k
}
