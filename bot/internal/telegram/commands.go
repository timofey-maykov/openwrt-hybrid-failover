package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/botconfig"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routing"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/validation"
)

type CommandHandler struct {
	routing routing.Service
	store  botconfig.Store
}

func NewCommandHandler(p routing.Service, s botconfig.Store) CommandHandler {
	return CommandHandler{routing: p, store: s}
}

func (h CommandHandler) Handle(ctx context.Context, _ int64, text string) (string, error) {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", fmt.Errorf("пустая команда")
	}

	switch fields[0] {
	case "/start", "/help":
		return h.helpText(), nil
	case "/quick", "/wizard":
		return h.quickGuideText(), nil
	case "/panel":
		return mainPanelText(), nil
	case "/uci_menu":
		return h.uciMenuText(), nil
	case "/param_menu":
		return h.paramMenuText(), nil
	case "/status":
		return h.routing.Status(ctx)
	case "/params", "/param_list":
		return h.routing.ListRouterParams(ctx)
	case "/uci_show":
		if len(fields) == 1 {
			return h.routing.ListRouterParams(ctx)
		}
		return h.routing.ShowRouterSection(ctx, fields[1])
	case "/uci_sections":
		raw, err := h.routing.ListRouterSections(ctx)
		if err != nil {
			return "", err
		}
		lines := strings.Split(strings.TrimSpace(raw), "\n")
		out := []string{"Секции hybrid-failover:"}
		for _, line := range lines {
			line = strings.TrimSpace(line)
			prefix := h.routing.UCIPackage() + "."
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
		return h.Handle(ctx, 0, "/param_get "+fields[1])
	case "/uci_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_set <hybrid-failover.section.option> <value>")
		}
		return h.Handle(ctx, 0, "/param_set "+fields[1]+" "+strings.Join(fields[2:], " "))
	case "/uci_add_list":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_add_list <hybrid-failover.section.option> <value>")
		}
		key := resolveParamKey(fields[1], h.routing.MainSection())
		val := strings.Join(fields[2:], " ")
		if err := h.routing.AddListRouterParam(ctx, key, val); err != nil {
			return "", err
		}
		return "Элемент добавлен в list (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/uci_del_list":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /uci_del_list <hybrid-failover.section.option> <value>")
		}
		key := resolveParamKey(fields[1], h.routing.MainSection())
		val := strings.Join(fields[2:], " ")
		if err := h.routing.DelListRouterParam(ctx, key, val); err != nil {
			return "", err
		}
		return "Элемент удален из list (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/uci_del":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /uci_del <hybrid-failover.section.option>")
		}
		return h.Handle(ctx, 0, "/param_del "+fields[1])
	case "/param_get":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /param_get <key>\nпример: /param_get disable_quic")
		}
		key := resolveParamKey(fields[1], h.routing.MainSection())
		value, err := h.routing.GetRouterParam(ctx, key)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s=%s", key, value), nil
	case "/param_set":
		if len(fields) < 3 {
			return "", fmt.Errorf("использование: /param_set <key> <value>\nпример: /param_set disable_quic on")
		}
		key := resolveParamKey(fields[1], h.routing.MainSection())
		value := strings.Join(fields[2:], " ")
		if err := h.routing.SetRouterParam(ctx, key, value); err != nil {
			return "", err
		}
		return "Параметр изменен в UCI (pending).\n1) /param_preview\n2) /param_apply\nили /param_rollback", nil
	case "/param_del":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /param_del <key>")
		}
		key := resolveParamKey(fields[1], h.routing.MainSection())
		if err := h.routing.DelRouterParam(ctx, key); err != nil {
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
		if err := h.routing.SetRouterParam(ctx, h.routing.SettingsKey("disable_quic"), value); err != nil {
			return "", err
		}
		return "QUIC обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/set_policy":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /set_policy outage-only|prefer-primary")
		}
		policy := strings.TrimSpace(fields[1])
		if policy != "outage-only" && policy != "prefer-primary" {
			return "", fmt.Errorf("допустимо только outage-only или prefer-primary")
		}
		if err := h.routing.SetRouterParam(ctx, h.routing.MainSectionKey("failover_policy"), policy); err != nil {
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
		if err := h.routing.SetRouterParam(ctx, h.routing.MainSectionKey("urltest_check_interval"), normalized); err != nil {
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
		if err := h.routing.SetRouterParam(ctx, h.routing.MainSectionKey("urltest_tolerance"), normalized); err != nil {
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
		if err := h.routing.SetRouterParam(ctx, h.routing.MainSectionKey("urltest_idle_timeout"), normalized); err != nil {
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
		if err := h.routing.SetRouterParam(ctx, h.routing.MainSectionKey("urltest_interrupt_exist_connections"), value); err != nil {
			return "", err
		}
		return "interrupt_exist_connections обновлен (pending). Проверьте /param_preview и примените /param_apply", nil
	case "/param_preview":
		return h.routing.PendingChanges(ctx)
	case "/param_apply":
		if err := h.routing.Apply(ctx); err != nil {
			return "", err
		}
		return "Изменения применены, сервис маршрутизации (init.d hybrid-failover) перезапущен", nil
	case "/param_rollback":
		if err := h.routing.Rollback(ctx); err != nil {
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
		return h.routing.Logs(ctx, lines)
	case "/channels", "/failover_list":
		health, err := h.routing.ChannelHealth(ctx)
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
		raw, err := h.routing.FailoverHistory(ctx)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(raw) == "" || raw == "[]" || raw == "null" {
			return "Событий failover пока нет.", nil
		}
		return "Последние события failover:\n" + raw, nil
	case "/health", "/check_channels":
		status, statusErr := h.routing.Status(ctx)
		health, err := h.routing.ChannelHealth(ctx)
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
			h.routing.MainSectionKey("failover_policy"),
			h.routing.MainSectionKey("urltest_check_interval"),
			h.routing.MainSectionKey("urltest_tolerance"),
			h.routing.MainSectionKey("urltest_idle_timeout"),
			h.routing.MainSectionKey("urltest_interrupt_exist_connections"),
		}
		out := []string{"Параметры failover:"}
		for _, key := range keys {
			val, err := h.routing.GetRouterParam(ctx, key)
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
		if err := h.routing.Restart(ctx); err != nil {
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
		if err := h.routing.AddFailover(ctx, uri); err != nil {
			return "", err
		}
		return "Резерв добавлен, примените /failover_apply", nil
	case "/failover_rm":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /failover_rm <uri>")
		}
		if err := h.routing.RemoveFailover(ctx, fields[1]); err != nil {
			return "", err
		}
		return "Резерв удален, примените /failover_apply", nil
	case "/failover_apply":
		if err := h.routing.Apply(ctx); err != nil {
			return "", err
		}
		return "Изменения применены (hybrid-failover)", nil
	case "/switch":
		if len(fields) < 2 {
			return "", fmt.Errorf("использование: /switch <outbound>")
		}
		if err := h.routing.SwitchOutbound(ctx, fields[1]); err != nil {
			return "", err
		}
		return "Переключение выполнено", nil
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

func (h CommandHandler) helpText() string {
	sec := h.routing.MainSection()
	sectionKey := h.routing.UCIPackage() + "." + sec
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

func mainPanelText() string {
	return "Панель Hybrid Failover. Выберите раздел кнопками ниже."
}

func (h CommandHandler) uciMenuText() string {
	return uciMenuTextForSection(h.routing.MainSection(), h.routing.UCIPackage())
}

func (h CommandHandler) UCISectionKey(option string) string {
	return uciSectionKey(h.routing.UCIPackage(), h.routing.MainSection(), option)
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

func (h CommandHandler) quickGuideText() string {
	sectionKey := h.routing.UCIPackage() + "." + h.routing.MainSection()
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

func (h CommandHandler) paramMenuText() string {
	sectionKey := h.routing.UCIPackage() + "." + h.routing.MainSection()
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
