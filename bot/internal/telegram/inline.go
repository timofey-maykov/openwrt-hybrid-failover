package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/tmaykov/openwrt-hybrid-failover/bot/internal/routers"
	"github.com/tmaykov/openwrt-hybrid-failover/internal/paths"
)

func routersPanelKeyboard(mgr *routers.Manager, userID int64) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	for _, r := range mgr.List() {
		label := r.Name
		if mgr.SelectedID(userID) == r.ID {
			label = "▶ " + label
		}
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(label, "cmd:/use "+r.ID),
		))
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Сервис", "nav:service"),
		tgbotapi.NewInlineKeyboardButtonData("Фейловер", "nav:failover"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Параметры", "nav:params"),
		tgbotapi.NewInlineKeyboardButtonData("Конфиг", "nav:config"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("UCI (hybrid-failover)", "nav:uci"),
	))
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Логи", "cmd:/logs 80"),
		tgbotapi.NewInlineKeyboardButtonData("Статус", "cmd:/status"),
	))
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func mainPanelKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сервис", "nav:service"),
			tgbotapi.NewInlineKeyboardButtonData("Фейловер", "nav:failover"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Параметры", "nav:params"),
			tgbotapi.NewInlineKeyboardButtonData("Конфиг", "nav:config"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("UCI (hybrid-failover)", "nav:uci"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Логи", "cmd:/logs 80"),
			tgbotapi.NewInlineKeyboardButtonData("Статус", "cmd:/status"),
		),
	)
}

func paramMenuKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Предпросмотр", "cmd:/param_preview"),
			tgbotapi.NewInlineKeyboardButtonData("Применить", "cmd:/param_apply"),
			tgbotapi.NewInlineKeyboardButtonData("Откат", "cmd:/param_rollback"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("QUIC ВЫКЛ", "cmd:/set_quic off"),
			tgbotapi.NewInlineKeyboardButtonData("QUIC ВКЛ", "cmd:/set_quic on"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Политика outage-only", "cmd:/set_policy outage-only"),
			tgbotapi.NewInlineKeyboardButtonData("Политика prefer-primary", "cmd:/set_policy prefer-primary"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Политика fastest", "cmd:/set_policy fastest"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("URLTest 30с", "cmd:/set_urltest_interval 30"),
			tgbotapi.NewInlineKeyboardButtonData("Список параметров", "cmd:/params"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav:main"),
		),
	)
}

func callbackToCommand(data string) (string, bool) {
	const prefix = "cmd:"
	if len(data) <= len(prefix) || data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

func callbackToNav(data string) (string, bool) {
	const prefix = "nav:"
	if len(data) <= len(prefix) || data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

func callbackToConfirm(data string) (string, bool) {
	const prefix = "confirm:"
	if len(data) <= len(prefix) || data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

func callbackToInput(data string) (string, bool) {
	const prefix = "input:"
	if len(data) <= len(prefix) || data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

func serviceKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Статус", "cmd:/status"),
			tgbotapi.NewInlineKeyboardButtonData("Перезапуск", "cmd:/routing_restart"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Логи", "cmd:/logs 100"),
			tgbotapi.NewInlineKeyboardButtonData("Каналы", "cmd:/channels"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Проверить доступность каналов", "cmd:/health"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav:main"),
		),
	)
}

func failoverKeyboard(section string) tgbotapi.InlineKeyboardMarkup {
	if section == "" {
		section = paths.DefaultMainSection
	}
	awgTag := section + "-awg-out"
	peerTag := section + "-1-out"
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Список", "cmd:/failover_list"),
			tgbotapi.NewInlineKeyboardButtonData("Параметры", "cmd:/failover_params"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Применить", "cmd:/failover_apply"),
			tgbotapi.NewInlineKeyboardButtonData("Справка", "cmd:/failover_help"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Primary VPN", "cmd:/switch "+awgTag),
			tgbotapi.NewInlineKeyboardButtonData("Backup #1", "cmd:/switch "+peerTag),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Интервал: ввести", "input:urltest_interval"),
			tgbotapi.NewInlineKeyboardButtonData("Tolerance: ввести", "input:urltest_tolerance"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Idle timeout: ввести", "input:urltest_idle_timeout"),
			tgbotapi.NewInlineKeyboardButtonData("Interrupt: on/off", "input:interrupt_existing"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav:main"),
		),
	)
}

func configKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Показать", "cmd:/config_show"),
			tgbotapi.NewInlineKeyboardButtonData("Проверить", "cmd:/config_validate"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Применить", "cmd:/config_apply"),
			tgbotapi.NewInlineKeyboardButtonData("Откат", "cmd:/config_rollback"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav:main"),
		),
	)
}

func confirmKeyboard(cmd string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", "confirm:"+cmd),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "nav:main"),
		),
	)
}

func inputCancelKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Отменить ввод", "input_cancel"),
		),
	)
}

func uciKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Показать всё", "cmd:/uci_show"),
			tgbotapi.NewInlineKeyboardButtonData("Секции", "cmd:/uci_sections"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("GET (ввод)", "input:uci_get"),
			tgbotapi.NewInlineKeyboardButtonData("SET (ввод)", "input:uci_set"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ADD_LIST (ввод)", "input:uci_add_list"),
			tgbotapi.NewInlineKeyboardButtonData("DEL_LIST (ввод)", "input:uci_del_list"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("DELETE key (ввод)", "input:uci_del"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Предпросмотр", "cmd:/param_preview"),
			tgbotapi.NewInlineKeyboardButtonData("Применить", "cmd:/param_apply"),
			tgbotapi.NewInlineKeyboardButtonData("Откат", "cmd:/param_rollback"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Назад", "nav:main"),
		),
	)
}
