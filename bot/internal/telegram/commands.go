package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/watchdog"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/routesreport"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

type CommandHandler struct {
	mgr      *routers.Manager
	store    botconfig.Store
	wd       *watchdog.Watchdog
	shellIDs map[int64]struct{}
}

// WithWatchdog attaches the watchdog whose state the monitoring screen shows.
func (h CommandHandler) WithWatchdog(w *watchdog.Watchdog) CommandHandler {
	h.wd = w
	return h
}

func NewCommandHandler(mgr *routers.Manager, s botconfig.Store) CommandHandler {
	return CommandHandler{mgr: mgr, store: s}
}

func (h CommandHandler) routingFor(userID int64) (routing.Service, error) {
	inst, err := h.mgr.InstanceFor(userID)
	if err != nil {
		return routing.Service{}, err
	}
	return inst.Service, nil
}

func (h CommandHandler) MainSectionFor(userID int64) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		return paths.DefaultMainSection
	}
	return rt.MainSection()
}

func (h CommandHandler) MainSection() string {
	return h.MainSectionFor(0)
}

func (h CommandHandler) listRouters(userID int64) string {
	list := h.mgr.List()
	if len(list) == 0 {
		return "Роутеры не настроены."
	}
	cur := h.mgr.SelectedID(userID)
	lines := []string{"Роутеры (Hybrid Failover):"}
	for _, r := range list {
		mark := " "
		if r.ID == cur || (cur == "" && len(list) == 1) {
			mark = "▶"
		}
		lines = append(lines, fmt.Sprintf("%s %s — %s (/use %s)", mark, r.ID, r.Name, r.ID))
	}
	if len(list) > 1 && cur == "" {
		lines = append(lines, "", "Выберите: /use <id>")
	}
	return strings.Join(lines, "\n")
}

func (h CommandHandler) Handle(ctx context.Context, userID int64, text string) (string, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", fmt.Errorf("пустая команда")
	}

	switch fields[0] {
	case "/routers":
		return h.listRouters(userID), nil
	case "/use":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /use <id>\nСписок: /routers")
		}
		if err := h.mgr.SetSelected(userID, fields[1]); err != nil {
			return "", err
		}
		inst, err := h.mgr.InstanceFor(userID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Активный роутер: %s (%s)", inst.Name, inst.ID), nil
	case "/router":
		id := h.mgr.SelectedID(userID)
		if id == "" {
			return h.listRouters(userID), nil
		}
		inst, err := h.mgr.InstanceFor(userID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Активный роутер: %s (%s)", inst.Name, inst.ID), nil
	}

	prefix := h.mgr.Prefix(userID)
	if fields[0] == "/sh" {
		resp, err := h.shell(ctx, userID, text)
		if err != nil {
			return "", err
		}
		return routing.MaskSecrets(prefix + resp), nil
	}
	resp, err := h.dispatch(ctx, userID, fields)
	if err != nil {
		return "", err
	}
	// Subscription and proxy links carry credentials; never echo them whole.
	return routing.MaskSecrets(prefix + resp), nil
}

func (h CommandHandler) dispatch(ctx context.Context, userID int64, fields []string) (string, error) {
	rt, err := h.routingFor(userID)
	if err != nil && !isBotLocalCommand(fields[0]) {
		return "", err
	}

	switch fields[0] {
	case "/start", "/help":
		return h.helpText(userID), nil
	case "/quick", "/wizard":
		return h.quickGuideText(userID), nil
	case "/panel":
		return mainPanelText(h.mgr), nil
	case "/uci_menu":
		return h.uciMenuText(userID), nil
	case "/param_menu":
		return h.paramMenuText(userID), nil
	case "/status":
		st, err := rt.Status(ctx)
		if err != nil {
			return "", err
		}
		if inst, ierr := h.mgr.InstanceFor(userID); ierr == nil && !h.mgr.Multi() {
			st = "Роутер: " + inst.Name + "\n" + st
		}
		return st, nil
	case "/params", "/param_list":
		return rt.ListRouterParams(ctx)
	case "/uci_show":
		if len(fields) == 1 {
			return rt.ListRouterParams(ctx)
		}
		return rt.ShowRouterSection(ctx, fields[1])
	case "/uci_sections":
		raw, err := rt.ListRouterSections(ctx)
		if err != nil {
			return "", err
		}
		lines := strings.Split(strings.TrimSpace(raw), "\n")
		out := []string{"Секции hybrid-failover:"}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			prefix := rt.UCIPackage() + "."
			if line == "" || !strings.HasPrefix(line, prefix) {
				continue
			}
			out = append(out, line)
		}
		return strings.Join(out, "\n"), nil
	case "/uci_get":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /uci_get <hybrid-failover.section.option>")
		}
		return h.dispatch(ctx, userID, []string{"/param_get", fields[1]})
	case "/uci_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_set <hybrid-failover.section.option> <value>")
		}
		return h.dispatch(ctx, userID, append([]string{"/param_set", fields[1]}, fields[2:]...))
	case "/uci_add_list":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_add_list <hybrid-failover.section.option> <value>")
		}
		key := resolveParamKey(fields[1], rt.MainSection())
		val := strings.Join(fields[2:], " ")
		if err := rt.AddListRouterParam(ctx, key, val); err != nil {
			return "", err
		}
		return "Элемент добавлен в list (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/uci_del_list":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_del_list <hybrid-failover.section.option> <value>")
		}
		key := resolveParamKey(fields[1], rt.MainSection())
		val := strings.Join(fields[2:], " ")
		if err := rt.DelListRouterParam(ctx, key, val); err != nil {
			return "", err
		}
		return "Элемент удален из list (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/uci_del":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /uci_del <hybrid-failover.section.option>")
		}
		return h.dispatch(ctx, userID, []string{"/param_del", fields[1]})
	case "/param_get":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /param_get <key>\nпример: /param_get disable_quic")
		}
		key := resolveParamKey(fields[1], rt.MainSection())
		value, err := rt.GetRouterParam(ctx, key)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", key, value), nil
	case "/param_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /param_set <key> <value>\nпример: /param_set disable_quic on")
		}
		key := resolveParamKey(fields[1], rt.MainSection())
		value := strings.Join(fields[2:], " ")
		if err := rt.SetRouterParam(ctx, key, value); err != nil {
			return "", err
		}
		return "Параметр изменен в UCI (pending).\n1) /param_preview\n2) /param_apply\nили /param_rollback", nil
	case "/param_del":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /param_del <key>")
		}
		key := resolveParamKey(fields[1], rt.MainSection())
		if err := rt.DelRouterParam(ctx, key); err != nil {
			return "", err
		}
		return "Параметр удален из UCI (pending).\n1) /param_preview\n2) /param_apply\nили /param_rollback", nil
	case "/set_quic":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_quic on|off")
		}
		// The argument is the state of QUIC itself; the option stores the opposite.
		value, err := quicStateToDisableValue(fields[1])
		if err != nil {
			return "", err
		}
		if err := rt.SetRouterParam(ctx, rt.SettingsKey("disable_quic"), value); err != nil {
			return "", err
		}
		return "QUIC обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_policy":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_policy outage-only|prefer-primary|fastest")
		}
		policy := strings.TrimSpace(fields[1])
		if policy != "outage-only" && policy != "prefer-primary" && policy != "fastest" {
			return "", fmt.Errorf("допустимо: outage-only, prefer-primary, fastest")
		}
		if err := rt.SetRouterParam(ctx, rt.MainSectionKey("failover_policy"), policy); err != nil {
			return "", err
		}
		return "Policy обновлена (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_urltest_interval":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_urltest_interval <seconds>")
		}
		normalized, err := parseDurationSeconds(fields[1])
		if err != nil {
			return "", err
		}
		if err := rt.SetRouterParam(ctx, rt.MainSectionKey("urltest_check_interval"), normalized); err != nil {
			return "", fmt.Errorf("%v\nПодсказка: idle_timeout должен быть ≥ interval (например interval 30s, idle 5m)", err)
		}
		return "URLTest check_interval обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_urltest_tolerance":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_urltest_tolerance <ms>")
		}
		normalized, err := parsePositiveInt(fields[1])
		if err != nil {
			return "", err
		}
		if err := rt.SetRouterParam(ctx, rt.MainSectionKey("urltest_tolerance"), normalized); err != nil {
			return "", err
		}
		return "URLTest tolerance обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_urltest_idle_timeout":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_urltest_idle_timeout <seconds>")
		}
		normalized, err := parseDurationSeconds(fields[1])
		if err != nil {
			return "", err
		}
		if err := rt.SetRouterParam(ctx, rt.MainSectionKey("urltest_idle_timeout"), normalized); err != nil {
			return "", fmt.Errorf("%v\nПодсказка: idle_timeout должен быть ≥ check_interval (сейчас часто 5m vs 60s)", err)
		}
		return "URLTest idle_timeout обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_interrupt_existing":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_interrupt_existing on|off")
		}
		value, err := onOffToBoolValue(fields[1])
		if err != nil {
			return "", err
		}
		if err := rt.SetRouterParam(ctx, rt.MainSectionKey("urltest_interrupt_exist_connections"), value); err != nil {
			return "", err
		}
		return "interrupt_exist_connections обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/param_preview":
		return rt.PendingChanges(ctx)
	case "/param_apply":
		if err := rt.Apply(ctx); err != nil {
			return "", err
		}
		return "Изменения применены, конфигурация движка перезагружена", nil
	case "/param_rollback":
		if err := rt.Rollback(ctx); err != nil {
			return "", err
		}
		return "Изменения параметров откатаны", nil
	case "/logs":
		lines := 50
		if len(fields) >= 2 {
			n, err := parsePositiveInt(fields[1])
			if err != nil {
				return "", fmt.Errorf("использование: /logs [lines]")
			}
			lines, _ = strconv.Atoi(n)
		}
		return rt.Logs(ctx, lines)
	case "/routes":
		secs, err := rt.ListRoutes(ctx)
		if err != nil {
			return "", err
		}
		return routing.FormatRoutes(secs), nil
	case "/route":
		return h.routeCommand(ctx, rt, fields)
	case "/channels", "/failover_list":
		health, err := rt.ChannelHealth(ctx)
		if err != nil {
			return "", err
		}
		if len(health) == 0 {
			return "Каналы не найдены.", nil
		}
		out := []string{"Каналы:"}
		for _, ch := range health {
			mark := "❌"
			if ch.Available {
				mark = "✅"
			}
			out = append(out, fmt.Sprintf("%s %s: %s", mark, ch.Name, ch.Detail))
		}
		return strings.Join(out, "\n"), nil
	case "/history", "/failover_history":
		raw, err := rt.FailoverHistory(ctx, 20)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(raw) == "" || raw == "[]" || raw == "null" {
			return "Событий failover пока нет.", nil
		}
		return "Последние события failover:\n" + raw, nil
	case "/health", "/check_channels":
		status, health, err := rt.Health(ctx)
		if err != nil {
			// Engine or core RPC unavailable: show what plain status still knows.
			st, statusErr := rt.Status(ctx)
			if statusErr != nil {
				return "", fmt.Errorf("%v. Также не удалось получить статус hybrid-failover: %v", err, statusErr)
			}
			return strings.Join([]string{
				"Проверка каналов временно недоступна.",
				"Причина: " + err.Error(),
				"",
				"Текущее состояние:",
				st,
				"",
				"Что сделать:",
				"1) /routing_restart",
				"2) подождать 5-10 сек",
				"3) /health",
			}, "\n"), nil
		}
		out := []string{}
		if strings.TrimSpace(status) != "" {
			out = append(out, "Состояние:", status, "")
		}
		if len(health) == 0 {
			out = append(out, "Каналы не найдены")
			return strings.Join(out, "\n"), nil
		}
		out = append(out, "Проверка каналов:")
		for _, ch := range health {
			mark := "❌"
			if ch.Available {
				mark = "✅"
			}
			out = append(out, fmt.Sprintf("%s %s: %s", mark, ch.Name, ch.Detail))
		}
		return strings.Join(out, "\n"), nil
	case "/failover_params":
		keys := []string{
			rt.MainSectionKey("failover_policy"),
			rt.MainSectionKey("urltest_check_interval"),
			rt.MainSectionKey("urltest_tolerance"),
			rt.MainSectionKey("urltest_idle_timeout"),
			rt.MainSectionKey("urltest_interrupt_exist_connections"),
		}
		out := []string{"Параметры failover:"}
		for _, key := range keys {
			val, err := rt.GetRouterParam(ctx, key)
			if err != nil {
				out = append(out, fmt.Sprintf("%s=<не задан>", key))
				continue
			}
			out = append(out, fmt.Sprintf("%s=%s", key, val))
		}
		return strings.Join(out, "\n"), nil
	case "/failover_help":
		return strings.Join([]string{
			"Редактирование failover:",
			"/failover_add <uri>",
			"/failover_rm <uri>",
			"/set_policy outage-only|prefer-primary|fastest",
			"/set_urltest_interval <sec>",
			"/set_urltest_tolerance <ms>",
			"/set_urltest_idle_timeout <sec>",
			"/set_interrupt_existing on|off",
			"/param_preview",
			"/param_apply",
		}, "\n"), nil
	case "/routing_restart", "/hybrid-failover_restart":
		if err := rt.Restart(ctx); err != nil {
			return "", err
		}
		return "Сервис маршрутизации (init.d hybrid-failover) перезапущен", nil
	case "/failover_add":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /failover_add <uri>")
		}
		uri := fields[1]
		if err := validation.ValidateProxyURI(uri); err != nil {
			return "", err
		}
		if err := rt.AddFailover(ctx, uri); err != nil {
			return "", err
		}
		return "Резерв добавлен, примените /failover_apply", nil
	case "/failover_rm":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /failover_rm <uri>")
		}
		if err := rt.RemoveFailover(ctx, fields[1]); err != nil {
			return "", err
		}
		return "Резерв удален, примените /failover_apply", nil
	case "/failover_apply":
		if err := rt.Apply(ctx); err != nil {
			return "", err
		}
		return "Изменения применены (hybrid-failover)", nil
	case "/switch":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /switch <outbound> или /switch <section> <outbound>")
		}
		section, outbound := "", fields[1]
		if len(fields) >= 3 {
			section, outbound = fields[1], fields[2]
		}
		if err := rt.SwitchOutbound(ctx, section, outbound); err != nil {
			return "", err
		}
		return "Переключение выполнено", nil
	case "/list_update":
		if err := rt.ListUpdate(ctx); err != nil {
			return "", err
		}
		return "Community lists обновлены", nil
	case "/subscription_refresh":
		if err := rt.SubscriptionRefresh(ctx); err != nil {
			return "", err
		}
		return "Подписки обновлены", nil
	case "/clients":
		out, err := rt.ListClients(ctx)
		if err != nil {
			return "", err
		}
		return out, nil
	case "/watch":
		return "Мониторинг: откройте /menu → 🛡 Мониторинг", nil
	case "/repair":
		return h.repair(ctx, userID)
	case "/mute":
		return h.mute(fields)
	case "/unmute":
		if h.wd == nil {
			return "", fmt.Errorf("мониторинг выключен")
		}
		h.wd.Unmute()
		return "Уведомления мониторинга включены", nil
	case "/sub_add":
		if len(fields) != 2 {
			return "", fmt.Errorf("использование: /sub_add <ссылка на подписку>")
		}
		if err := rt.AddSubscription(ctx, fields[1]); err != nil {
			return "", err
		}
		return "Подписка добавлена (pending). Примените изменения: /param_apply", nil
	case "/sub_del":
		n, err := argInt(fields, 1, "использование: /sub_del <номер>")
		if err != nil {
			return "", err
		}
		u, err := rt.DelSubscription(ctx, n)
		if err != nil {
			return "", err
		}
		return "Подписка удалена (pending): " + routing.MaskURL(u), nil
	case "/sub_interval":
		if len(fields) != 2 {
			return "", fmt.Errorf("использование: /sub_interval <1h|6h|12h|24h>")
		}
		if err := rt.SetSubscriptionInterval(ctx, fields[1]); err != nil {
			return "", err
		}
		return "Интервал обновления подписок сохранён (pending)", nil
	case "/rt":
		return h.routeByIndex(ctx, rt, fields)
	case "/config_show":
		cfg, err := h.store.LoadPending()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("policy=%s\nclash_api=%s\nlog_path=%s\naudit_path=%s", cfg.Policy, cfg.ClashAPI, cfg.LogPath, cfg.AuditPath), nil
	case "/config_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /config_set <key> <value>")
		}
		if err := h.store.SetPendingKey(fields[1], strings.Join(fields[2:], " ")); err != nil {
			return "", err
		}
		return "Значение записано в pending-конфиг", nil
	case "/config_validate":
		if err := h.store.ValidatePending(); err != nil {
			return "", err
		}
		diff, err := h.store.DiffSummary()
		if err != nil {
			return "Pending валиден", nil
		}
		return "Pending валиден\n" + diff, nil
	case "/config_apply":
		if err := h.store.ApplyPending(); err != nil {
			return "", err
		}
		return "Pending-конфиг записан. Бот читает конфиг при старте, перезапустите его: /etc/init.d/hybrid-failover-bot restart", nil
	case "/config_rollback":
		if err := h.store.RollbackPending(); err != nil {
			return "", err
		}
		return "Pending-конфиг откатан", nil
	default:
		if resp, handled, err := h.dispatchRouter(ctx, userID, fields); handled {
			return resp, err
		}
		return "", fmt.Errorf("неизвестная команда: %s", fields[0])
	}
}

func isBotLocalCommand(cmd string) bool {
	switch cmd {
	case "/config_show", "/config_set", "/config_validate", "/config_apply", "/config_rollback",
		"/start", "/help", "/routers", "/use", "/router", "/panel",
		"/quick", "/wizard", "/uci_menu", "/param_menu":
		return true
	default:
		return false
	}
}

func (h CommandHandler) helpText(userID int64) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		base := helpText()
		return base + "\n\n⚠ " + err.Error()
	}
	sec := rt.MainSection()
	sectionKey := rt.UCIPackage() + "." + sec
	lines := []string{
		"Команды:",
	}
	if h.mgr.Multi() {
		lines = append(lines, "/routers — список роутеров", "/use <id> — выбрать роутер", "/router — текущий роутер", "")
	}
	lines = append(lines,
		"Быстрый старт:",
		"/quick",
		"/panel",
		"/param_menu",
		"/uci_menu",
		"/status",
		"/params",
		"/uci_show [hybrid-failover.section]",
		"/uci_sections",
		"/uci_get <key>",
		"/uci_set <key> <value>",
		"/uci_add_list <key> <value>",
		"/uci_del_list <key> <value>",
		"/uci_del <key>",
		"/param_list",
		"/param_get <key|alias>",
		"/param_set <key|alias> <value>",
		"/param_del <key|alias>",
		"/param_preview",
		"/param_apply",
		"/param_rollback",
		"/set_quic on|off",
		"/set_policy outage-only|prefer-primary|fastest",
		"/set_urltest_interval <seconds>",
		"/set_urltest_tolerance <ms>",
		"/set_urltest_idle_timeout <seconds>",
		"/set_interrupt_existing on|off",
		"/channels",
		"/routes",
		"/route <список> <канал> [pool|direct|block]",
		"/health",
		"/check_channels",
		"/routing_restart",
		"/failover_list",
		"/failover_params",
		"/failover_help",
		"/failover_add <uri>",
		"/failover_rm <uri>",
		"/failover_apply",
		"/switch <outbound>",
		"/history",
		"/clients",
		"/list_update",
		"/subscription_refresh",
		"/logs [lines]",
		"/config_show",
		"/config_set <key> <value>",
		"/config_validate",
		"/config_apply",
		"/config_rollback",
		"",
	)
	lines = append(lines, routerHelpLines()...)
	lines = append(lines, "", "Основная секция UCI: "+sectionKey)
	return strings.Join(lines, "\n")
}

func helpText() string {
	sec := paths.DefaultMainSection
	sectionKey := paths.UCIPackage + "." + sec
	return strings.Join([]string{
		"Команды:",
		"Быстрый старт:",
		"/quick",
		"/panel",
		"/param_menu",
		"/uci_menu",
		"/status",
		"/params",
		"/uci_show [hybrid-failover.section]",
		"/uci_sections",
		"/uci_get <key>",
		"/uci_set <key> <value>",
		"/uci_add_list <key> <value>",
		"/uci_del_list <key> <value>",
		"/uci_del <key>",
		"/param_list",
		"/param_get <key|alias>",
		"/param_set <key|alias> <value>",
		"/param_del <key|alias>",
		"/param_preview",
		"/param_apply",
		"/param_rollback",
		"/set_quic on|off",
		"/set_policy outage-only|prefer-primary|fastest",
		"/set_urltest_interval <seconds>",
		"/set_urltest_tolerance <ms>",
		"/set_urltest_idle_timeout <seconds>",
		"/set_interrupt_existing on|off",
		"/channels",
		"/routes",
		"/route <список> <канал> [pool|direct|block]",
		"/health",
		"/check_channels",
		"/routing_restart",
		"/failover_list",
		"/failover_params",
		"/failover_help",
		"/failover_add <uri>",
		"/failover_rm <uri>",
		"/failover_apply",
		"/switch <outbound>",
		"/history",
		"/clients",
		"/list_update",
		"/subscription_refresh",
		"/logs [lines]",
		"/config_show",
		"/config_set <key> <value>",
		"/config_validate",
		"/config_apply",
		"/config_rollback",
		"",
		"Основная секция UCI: " + sectionKey,
	}, "\n") + "\n\n" + strings.Join(routerHelpLines(), "\n")
}

func mainPanelText(mgr *routers.Manager) string {
	if mgr != nil && mgr.Multi() {
		return "Панель Hybrid Failover. Выберите роутер: /routers → /use <id>. Затем раздел кнопками ниже."
	}
	if mgr != nil {
		if list := mgr.List(); len(list) == 1 {
			return "Панель Hybrid Failover · роутер " + list[0].Name + ". Выберите раздел кнопками ниже."
		}
	}
	return "Панель Hybrid Failover. Выберите раздел кнопками ниже."
}

func (h CommandHandler) uciMenuText(userID int64) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		return uciMenuText() + "\n\n⚠ " + err.Error()
	}
	return uciMenuTextForSection(rt.MainSection(), rt.UCIPackage())
}

func (h CommandHandler) UCISectionKey(option string) string {
	return uciSectionKey(paths.UCIPackage, h.MainSection(), option)
}

func uciMenuText() string {
	return uciMenuTextForSection(paths.DefaultMainSection, paths.UCIPackage)
}

func uciMenuTextForSection(sec, pkg string) string {
	sectionKey := pkg + "." + sec
	return strings.Join([]string{
		"UCI конфигурация upstream hybrid-failover:",
		"",
		"Просмотр:",
		"/uci_show",
		"/uci_sections",
		"/uci_show " + sectionKey,
		"",
		"Редактирование:",
		"/uci_get " + sectionKey + ".urltest_check_interval",
		"/uci_set " + sectionKey + ".urltest_check_interval 45s",
		"/uci_add_list " + sectionKey + ".failover_proxy_links vless://...",
		"/uci_del_list " + sectionKey + ".failover_proxy_links vless://...",
		"/uci_del " + sectionKey + ".urltest_tolerance",
		"",
		"Фиксация изменений:",
		"/param_preview",
		"/param_apply",
		"/param_rollback",
	}, "\n")
}

func (h CommandHandler) quickGuideText(userID int64) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		return quickGuideText() + "\n\n⚠ " + err.Error()
	}
	sectionKey := rt.UCIPackage() + "." + rt.MainSection()
	return strings.Join([]string{
		"Удобные сценарии:",
		"",
		"1) Выключить QUIC:",
		"/set_quic off",
		"/param_preview",
		"/param_apply",
		"",
		"2) Поменять политику failover:",
		"/set_policy outage-only",
		"/param_preview",
		"/param_apply",
		"",
		"3) Изменить любой параметр вручную:",
		"/param_set urltest_tolerance 100",
		"/param_preview",
		"/param_apply",
		"",
		"Алиасы ключей: disable_quic, urltest_interval, urltest_tolerance, policy",
		"Основная секция: " + sectionKey,
	}, "\n")
}

func quickGuideText() string {
	return strings.Join([]string{
		"Удобные сценарии:",
		"",
		"1) Выключить QUIC:",
		"/set_quic off",
		"/param_preview",
		"/param_apply",
		"",
		"2) Поменять политику failover:",
		"/set_policy outage-only",
		"/param_preview",
		"/param_apply",
		"",
		"3) Изменить любой параметр вручную:",
		"/param_set urltest_tolerance 100",
		"/param_preview",
		"/param_apply",
		"",
		"Алиасы ключей: disable_quic, urltest_interval, urltest_tolerance, policy",
	}, "\n")
}

func (h CommandHandler) paramMenuText(userID int64) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		return paramMenuText() + "\n\n⚠ " + err.Error()
	}
	sectionKey := rt.UCIPackage() + "." + rt.MainSection()
	return strings.Join([]string{
		"Меню параметров роутера (конфиг hybrid-failover):",
		"",
		"1) Показать все параметры:",
		"   /params",
		"",
		"2) Проверить конкретный параметр:",
		"   /param_get disable_quic",
		"   /param_get hybrid-failover.settings.disable_quic",
		"",
		"3) Выключить QUIC (рекомендуется для проблемного YouTube):",
		"   /set_quic off",
		"",
		"4) Настроить политику failover:",
		"   /set_policy outage-only",
		"   /set_policy prefer-primary",
		"",
		"5) Изменить интервал URLTest:",
		"   /set_urltest_interval 30   (сохранит как 30s в urltest_check_interval)",
		"",
		"6) Ручная правка любого hybrid-failover-параметра:",
		"   /param_set urltest_tolerance 100",
		"   /param_del " + sectionKey + ".urltest_tolerance",
		"",
		"7) Перед применением обязательно посмотреть diff:",
		"   /param_preview",
		"",
		"8) Применить или откатить:",
		"   /param_apply",
		"   /param_rollback",
		"",
		"Короткие алиасы ключей: disable_quic, urltest_interval,",
		"urltest_tolerance, urltest_idle_timeout, urltest_interrupt_exist_connections, policy",
		"Основная секция: " + sectionKey,
	}, "\n")
}

func paramMenuText() string {
	sectionKey := paths.UCIPackage + "." + paths.DefaultMainSection
	return strings.Join([]string{
		"Меню параметров роутера (конфиг hybrid-failover):",
		"",
		"1) Показать все параметры:",
		"   /params",
		"",
		"2) Проверить конкретный параметр:",
		"   /param_get disable_quic",
		"   /param_get hybrid-failover.settings.disable_quic",
		"",
		"3) Выключить QUIC (рекомендуется для проблемного YouTube):",
		"   /set_quic off",
		"",
		"4) Настроить политику failover:",
		"   /set_policy outage-only",
		"   /set_policy prefer-primary",
		"",
		"5) Изменить интервал URLTest:",
		"   /set_urltest_interval 30   (сохранит как 30s в urltest_check_interval)",
		"",
		"6) Ручная правка любого hybrid-failover-параметра:",
		"   /param_set urltest_tolerance 100",
		"   /param_del " + sectionKey + ".urltest_tolerance",
		"",
		"7) Перед применением обязательно посмотреть diff:",
		"   /param_preview",
		"",
		"8) Применить или откатить:",
		"   /param_apply",
		"   /param_rollback",
		"",
		"Короткие алиасы ключей: disable_quic, urltest_interval,",
		"urltest_tolerance, urltest_idle_timeout, urltest_interrupt_exist_connections, policy",
	}, "\n")
}

// routeCommand: /route <list> <channel> [on_down] [section]. The section is
// optional when the list exists in one section only.
func (h CommandHandler) routeCommand(ctx context.Context, rt routing.Service, fields []string) (string, error) {
	if len(fields) < 3 {
		return "", fmt.Errorf("использование: /route <список> <номер канала|pool|balance|direct|block> [pool|direct|block] [секция]\nсписки и номера каналов: /routes")
	}
	listKey, chanArg := fields[1], fields[2]
	onDown, secName := "pool", ""
	for _, f := range fields[3:] {
		switch f {
		case "pool", "direct", "block":
			onDown = f
		default:
			secName = f
		}
	}
	secs, err := rt.ListRoutes(ctx)
	if err != nil {
		return "", err
	}
	var found []routesreport.Section
	for _, sec := range secs {
		if secName != "" && sec.Name != secName {
			continue
		}
		for _, l := range sec.Lists {
			if l.Key == listKey || l.Key == "user:"+listKey || (l.Kind == "user_list" && strings.EqualFold(l.Title, listKey)) {
				listKey = l.Key
				found = append(found, sec)
				break
			}
		}
	}
	switch {
	case len(found) == 0:
		return "", fmt.Errorf("список «%s» не найден, смотрите /routes", fields[1])
	case len(found) > 1:
		return "", fmt.Errorf("список «%s» есть в нескольких секциях, добавьте имя секции в конце команды", fields[1])
	}
	sec := found[0]
	ch, err := routing.ResolveChannel(sec, chanArg)
	if err != nil {
		return "", err
	}
	if err := rt.SetListRoute(ctx, sec.Name, listKey, ch, onDown); err != nil {
		return "", err
	}
	target := ch
	for _, c := range sec.Channels {
		if c.ID == ch {
			target = c.Name
		}
	}
	return fmt.Sprintf("%s → %s (pending).\nПроверьте /param_preview и примените /param_apply", listKey, target), nil
}

func argInt(fields []string, i int, usage string) (int, error) {
	if len(fields) <= i {
		return 0, fmt.Errorf("%s", usage)
	}
	n, err := strconv.Atoi(fields[i])
	if err != nil {
		return 0, fmt.Errorf("%s", usage)
	}
	return n, nil
}

// routeByIndex is /rt <list position> <channel>: the button form of /route.
// The position is the one the Lists screen shows, so it is resolved against a
// fresh report and rejected if the lists changed meanwhile.
func (h CommandHandler) routeByIndex(ctx context.Context, rt routing.Service, fields []string) (string, error) {
	idx, err := argInt(fields, 1, "использование: /rt <номер списка> <канал>")
	if err != nil {
		return "", err
	}
	if len(fields) < 3 {
		return "", fmt.Errorf("использование: /rt <номер списка> <канал>")
	}
	secs, err := rt.ListRoutes(ctx)
	if err != nil {
		return "", err
	}
	flat := routing.FlattenLists(secs)
	if idx < 0 || idx >= len(flat) {
		return "", fmt.Errorf("список не найден, обновите экран")
	}
	f := flat[idx]
	ch, err := routing.ResolveChannel(f.Section, fields[2])
	if err != nil {
		return "", err
	}
	if err := rt.SetListRoute(ctx, f.Section.Name, f.List.Key, ch, "pool"); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s → %s (pending)", routing.ListTitle(f.List), routing.ChannelLabel(f.Section, ch)), nil
}

func (h CommandHandler) repair(ctx context.Context, userID int64) (string, error) {
	if h.wd == nil {
		return "", fmt.Errorf("мониторинг выключен в конфиге бота (watchdog_enabled)")
	}
	inst, err := h.mgr.InstanceFor(userID)
	if err != nil {
		return "", err
	}
	if _, err := h.wd.RepairNow(ctx, inst.ID); err != nil {
		return "", err
	}
	return "Сервис перезапущен. Состояние проверится на следующем цикле.", nil
}

// mute silences watchdog notices: /mute <minutes>, default one hour.
func (h CommandHandler) mute(fields []string) (string, error) {
	if h.wd == nil {
		return "", fmt.Errorf("мониторинг выключен в конфиге бота (watchdog_enabled)")
	}
	minutes := 60
	if len(fields) >= 2 {
		n, err := strconv.Atoi(fields[1])
		if err != nil || n < 1 || n > 24*60 {
			return "", fmt.Errorf("использование: /mute <минуты, 1..1440>")
		}
		minutes = n
	}
	h.wd.Mute(time.Duration(minutes) * time.Minute)
	return fmt.Sprintf("Уведомления мониторинга выключены на %d мин. Починка продолжает работать.", minutes), nil
}
