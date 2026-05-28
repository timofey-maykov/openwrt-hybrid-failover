package telegram

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
			tgbotapi.NewInlineKeyboardButtonData("UCI Podkop", "nav:uci"),
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
			tgbotapi.NewInlineKeyboardButtonData("Перезапуск", "cmd:/podkop_restart"),
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

func failoverKeyboard() tgbotapi.InlineKeyboardMarkup {
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
			tgbotapi.NewInlineKeyboardButtonData("Переключить AWG", "cmd:/switch glob-awg-out"),
			tgbotapi.NewInlineKeyboardButtonData("Переключить #1", "cmd:/switch glob-1-out"),
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
