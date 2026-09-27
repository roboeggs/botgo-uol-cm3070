package handlers

import (
	"log/slog"

	"botgo/internal/database"
	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

func Handlestart(c tele.Context) error {
	userID := c.Sender().ID

	lang, err := database.UpsertUser(c)
	if err != nil {
		slog.Error("UpsertUser in /start failed", "user_id", userID, "error", err)
	}
	i18n.SetLanguage(userID, lang)

	name := c.Sender().FirstName
	if name == "" {
		name = c.Sender().Username
	}
	if name == "" {
		name = "friend"
	}

	text := i18n.T(userID, "greeting", map[string]interface{}{"Name": name})
	menu := InitMainMenu(webAppBaseURL, lang)
	return c.Send(text, menu)
}

func HandleHelp(c tele.Context) error {
	userID := c.Sender().ID
	text := i18n.T(userID, "help_message")
	return reply(c, text)
}

func HandleLanguageRu(c tele.Context) error { return handleLanguageChange(c, "ru") }
func HandleLanguageEn(c tele.Context) error { return handleLanguageChange(c, "en") }

func handleLanguageChange(c tele.Context, lang string) error {
	defer c.Respond(&tele.CallbackResponse{})
	userID := c.Sender().ID

	if err := database.UpdateLanguage(userID, lang); err != nil {
		slog.Error("UpdateLanguage failed", "user_id", userID, "lang", lang, "error", err)
	}
	i18n.SetLanguage(userID, lang)

	// Delete the message with inline language selection buttons
	_ = c.Delete()

	// New message + reply keyboard for the new language
	menu := InitMainMenu(webAppBaseURL, lang)
	return c.Send(i18n.T(userID, "lang_changed"), menu)
}

func HandleUnknownCallback(c tele.Context) error {
	defer c.Respond(&tele.CallbackResponse{})
	userID := c.Sender().ID
	return reply(c, i18n.T(userID, "unknown_action"))
}

// Temporary handler to reset the keyboard
func HandleResetKeyboard(c tele.Context) error {
    return c.Send("Keyboard reset", &tele.ReplyMarkup{RemoveKeyboard: true})
}