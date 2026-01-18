package menu

import (
	"lunobot/i18n"
	"lunobot/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MenuButton struct {
	TextKey  string
	Callback string
}

type MenuGenerator struct {
	buttons    map[models.Rights][]MenuButton
	translator *i18n.Translator
}

func NewMenuGenerator(translator *i18n.Translator) *MenuGenerator {
	mg := &MenuGenerator{
		buttons:    make(map[models.Rights][]MenuButton),
		translator: translator,
	}
	mg.initializeButtons()
	return mg
}

func (mg *MenuGenerator) initializeButtons() {
	mg.buttons[models.RightsDefault] = []MenuButton{
		{TextKey: "btn_check_status", Callback: "check_status"},
		{TextKey: "btn_send_idea", Callback: "send_idea"},
		{TextKey: "btn_notifications", Callback: "notifications"},
		{TextKey: "btn_language", Callback: "change_language"},
	}
	mg.buttons[models.RightsManager] = []MenuButton{
		{TextKey: "btn_check_status", Callback: "check_status"},
		{TextKey: "btn_send_idea", Callback: "send_idea"},
		{TextKey: "btn_notifications", Callback: "notifications"},
		{TextKey: "btn_language", Callback: "change_language"},
		{TextKey: "btn_set_open_status", Callback: "set_open_status"},
		{TextKey: "btn_set_tech_status", Callback: "set_tech_status"},
	}
	mg.buttons[models.RightsAdmin] = []MenuButton{
		{TextKey: "btn_check_status", Callback: "check_status"},
		{TextKey: "btn_send_idea", Callback: "send_idea"},
		{TextKey: "btn_notifications", Callback: "notifications"},
		{TextKey: "btn_language", Callback: "change_language"},
		{TextKey: "btn_set_open_status", Callback: "set_open_status"},
		{TextKey: "btn_set_tech_status", Callback: "set_tech_status"},
		{TextKey: "btn_read_ideas", Callback: "read_ideas"},
		{TextKey: "btn_set_rights", Callback: "set_rights"},
		{TextKey: "btn_create_broadcast", Callback: "create_broadcast"},
		{TextKey: "btn_auto_close", Callback: "auto_close"},
	}
}

func (mg *MenuGenerator) GenerateKeyboard(userRights models.Rights, lang i18n.Language) tgbotapi.InlineKeyboardMarkup {
	buttons := mg.getButtonsForRights(userRights)
	var rows [][]tgbotapi.InlineKeyboardButton
	for i := 0; i < len(buttons); i += 2 {
		var row []tgbotapi.InlineKeyboardButton

		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			mg.translator.Get(buttons[i].TextKey, lang), buttons[i].Callback,
		))

		if i+1 < len(buttons) {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				mg.translator.Get(buttons[i+1].TextKey, lang), buttons[i+1].Callback,
			))
		}

		rows = append(rows, row)
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func (mg *MenuGenerator) getButtonsForRights(rights models.Rights) []MenuButton {
	if buttons, exists := mg.buttons[rights]; exists {
		return buttons
	}
	return mg.buttons[models.RightsDefault]
}

func (mg *MenuGenerator) GenerateLanguageKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇺🇦 Українська", "lang_ua"),
			tgbotapi.NewInlineKeyboardButtonData("🇬🇧 English", "lang_en"),
		),
	)
}

func (mg *MenuGenerator) GenerateStatusKeyboard(statusType string, lang i18n.Language) tgbotapi.InlineKeyboardMarkup {
	if statusType == "tech" {
		return tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("status_btn_keys_admin", lang), statusType+"_true"),
				tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("status_btn_keys_lobby", lang), statusType+"_false"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_back", lang), "back_to_menu"),
			),
		)
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("status_btn_open", lang), statusType+"_true"),
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("status_btn_close", lang), statusType+"_false"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_back", lang), "back_to_menu"),
		),
	)
}

func (mg *MenuGenerator) GenerateRightsKeyboard(lang i18n.Language) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_rights_user", lang), "rights_1"),
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_rights_manager", lang), "rights_2"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_rights_admin", lang), "rights_3"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_back", lang), "back_to_menu"),
		),
	)
}

func (mg *MenuGenerator) GenerateNotificationsKeyboard(enabled bool, lang i18n.Language) tgbotapi.InlineKeyboardMarkup {
	var toggleButton tgbotapi.InlineKeyboardButton
	if enabled {
		toggleButton = tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_notifications_disable", lang), "notifications_toggle")
	} else {
		toggleButton = tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_notifications_enable", lang), "notifications_toggle")
	}

	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(toggleButton),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(mg.translator.Get("btn_back", lang), "back_to_menu"),
		),
	)
}
