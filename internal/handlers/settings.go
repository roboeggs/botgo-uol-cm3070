package handlers

import (
	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

func HandleSettings(c tele.Context) error {
	userID := c.Sender().ID
	text := i18n.T(userID, "settings_title")
	return c.Send(text, LangMenu)
}