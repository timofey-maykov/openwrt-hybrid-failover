package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

func callbackToConfirmToken(data string) (string, bool) {
	const prefix = "cfm:"
	if len(data) <= len(prefix) || data[:len(prefix)] != prefix {
		return "", false
	}
	return data[len(prefix):], true
}

func confirmTokenKeyboard(id string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", "cfm:"+id),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "nav:router"),
		),
	)
}
