package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

type CommandHandler struct {
	mgr   *routers.Manager
	store botconfig.Store
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
	resp, err := h.dispatch(ctx, userID, fields)
	if err != nil {
		return "", err
	}
	return prefix + resp, nil
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
		return rt.Status(ctx)
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
		return h.Handle(ctx, userID, "/param_get "+fields[1])
	case "/uci_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_set <hybrid-failover.section.option> <value>")
		}
		return h.Handle(ctx, userID, "/param_set "+fields[1]+" "+strings.Join(fields[2:], " "))
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
		return h.Handle(ctx, userID, "/param_del "+fields[1])
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
		value, err := onOffToBoolValue(fields[1])
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
		return "Изменения применены, сервис маршрутизации (init.d hybrid-failover) перезапущен", nil
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
		raw, err := rt.FailoverHistory(ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(raw) == "" || raw == "[]" || raw == "null" {
			return "Событий failover пока нет.", nil
		}
		return "Последние события failover:\n" + raw, nil
	case "/health", "/check_channels":
		status, statusErr := rt.Status(ctx)
		health, err := rt.ChannelHealth(ctx)
		if err != nil {
			if statusErr != nil {
				return "", fmt.Errorf("%v. Также не удалось получить статус hybrid-failover: %v", err, statusErr)
			}
			out := []string{
				"Проверка каналов временно недоступна.",
				"Причина: " + err.Error(),
				"",
				"Текущее состояние:",
				status,
				"",
				"Что сделать:",
				"1) /routing_restart",
				"2) подождать 5-10 сек",
				"3) /health",
			}
			return strings.Join(out, "\n"), nil
		}
		out := []string{}
		if statusErr == nil && strings.TrimSpace(status) != "" {
			out = append(out, "Состояние:", status, "")
		}
		if len(health) == 0 {
			out = append(out, "Каналы не найдены")
			return strings.Join(out, "\n"), nil
		}
		out = append(out, "Проверка каналов:")
		for _, ch := range health {
			if ch.Available {
				out = append(out, fmt.Sprintf("✅ %s: %s", ch.Name, ch.Detail))
			} else {
				out = append(out, fmt.Sprintf("❌ %s: %s", ch.Name, ch.Detail))
			}
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
			"/set_policy outage-only|prefer-primary",
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
		if err := h.store.SetPendingKey(fields[1], fields[2]); err != nil {
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
		return "Pending-конфиг применен", nil
	case "/config_rollback":
		if err := h.store.RollbackPending(); err != nil {
			return "", err
		}
		return "Pending-конфиг откатан", nil
	default:
		return "", fmt.Errorf("неизвестная команда: %s", fields[0])
	}
}

func isBotLocalCommand(cmd string) bool {
	switch cmd {
	case "/config_show", "/config_set", "/config_validate", "/config_apply", "/config_rollback",
		"/start", "/help", "/routers", "/use", "/router", "/panel":
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
		"/set_policy outage-only|prefer-primary",
		"/set_urltest_interval <seconds>",
		"/set_urltest_tolerance <ms>",
		"/set_urltest_idle_timeout <seconds>",
		"/set_interrupt_existing on|off",
		"/channels",
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
		"/logs [lines]",
		"/config_show",
		"/config_set <key> <value>",
		"/config_validate",
		"/config_apply",
		"/config_rollback",
		"",
		"Основная секция UCI: " + sectionKey,
	)
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
		"/set_policy outage-only|prefer-primary",
		"/set_urltest_interval <seconds>",
		"/set_urltest_tolerance <ms>",
		"/set_urltest_idle_timeout <seconds>",
		"/set_interrupt_existing on|off",
		"/channels",
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
		"/logs [lines]",
		"/config_show",
		"/config_set <key> <value>",
		"/config_validate",
		"/config_apply",
		"/config_rollback",
		"",
		"Основная секция UCI: " + sectionKey,
	}, "\n")
}

func mainPanelText(mgr *routers.Manager) string {
	if mgr != nil && mgr.Multi() {
		return "Панель Hybrid Failover. Выберите роутер: /routers → /use <id>. Затем раздел кнопками ниже."
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

func (h CommandHandler) uciSectionKeyFor(userID int64, option string) string {
	rt, err := h.routingFor(userID)
	if err != nil {
		return uciSectionKey(paths.UCIPackage, paths.DefaultMainSection, option)
	}
	return uciSectionKey(rt.UCIPackage(), rt.MainSection(), option)
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
		"/param_set hybrid-failover.settings.cache_path /etc/sing-box/cache.db",
		"/param_preview",
		"/param_apply",
		"",
		"Алиасы ключей: disable_quic, cache_path, urltest_interval, urltest_tolerance",
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
		"/param_set hybrid-failover.settings.cache_path /etc/sing-box/cache.db",
		"/param_preview",
		"/param_apply",
		"",
		"Алиасы ключей: disable_quic, cache_path, urltest_interval, urltest_tolerance",
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
		"   /param_set hybrid-failover.settings.cache_path /etc/sing-box/cache.db",
		"   /param_del " + sectionKey + ".urltest_tolerance",
		"",
		"7) Перед применением обязательно посмотреть diff:",
		"   /param_preview",
		"",
		"8) Применить или откатить:",
		"   /param_apply",
		"   /param_rollback",
		"",
		"Короткие алиасы ключей: disable_quic, cache_path, urltest_interval,",
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
		"   /param_set hybrid-failover.settings.cache_path /etc/sing-box/cache.db",
		"   /param_del " + sectionKey + ".urltest_tolerance",
		"",
		"7) Перед применением обязательно посмотреть diff:",
		"   /param_preview",
		"",
		"8) Применить или откатить:",
		"   /param_apply",
		"   /param_rollback",
		"",
		"Короткие алиасы ключей: disable_quic, cache_path, urltest_interval,",
		"urltest_tolerance, urltest_idle_timeout, urltest_interrupt_exist_connections, policy",
	}, "\n")
}
