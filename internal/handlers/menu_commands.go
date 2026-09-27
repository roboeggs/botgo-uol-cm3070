package handlers

import (
	"net/url"
	"strings"

	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

// HandleMenuAction turns the slash command into the same action as the reply button.
// The same logic applies to buttons and to /market, /favorites, /screener, /settings —
// so that the behavior doesn’t diverge.
func HandleMenuAction(action string) tele.HandlerFunc {
	return func(c tele.Context) error {
		switch action {

		case actMarket:
			return handleMarketOpen(c)

		case actFav:
			return HandleFavorites(c)

		case actScreener:
			return handleScreenerAction(c)

		case actSettings:
			return HandleSettings(c)

		default:
			userID := c.Sender().ID
			return reply(c, i18n.T(userID, "unknown_action"))
		}
	}
}

// handleScreenerAction — opens the WebApp with an inline button.
// Telegram doesn’t allow a slash command to open the WebApp directly, so
// we send a message with an inline WebApp button (the same link as
// the reply button in InitMainMenu).
func handleScreenerAction(c tele.Context) error {
	userID := c.Sender().ID
	lang := i18n.GetLanguage(userID)

	if webAppBaseURL == "" {
		return reply(c, i18n.T(userID, "webapp_not_configured"))
	}

	screenerURL := strings.TrimRight(webAppBaseURL, "/") +
		"/static/chart.html?mode=market&lang=" + url.QueryEscape(lang)

	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.WebApp(i18n.TByLang(lang, "btn_screener"), &tele.WebApp{URL: screenerURL}),
	))

	return reply(c, i18n.T(userID, "market_hint"), menu)
}