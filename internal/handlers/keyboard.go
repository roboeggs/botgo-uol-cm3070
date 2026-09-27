package handlers

import (
	"strings"
	"net/url"

	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

// Actions for reply buttons.
const (
	actMarket   = "market"
	actFav      = "favorites"
	actScreener = "screener"
	actSettings = "settings"
)

// buttonKeys — a map: i18n key -> action.
var buttonKeys = map[string]string{
	"btn_market":    actMarket,
	"btn_favorites": actFav,
	"btn_screener":  actScreener,
	"btn_settings":  actSettings,
}

// buttonActions — reverse lookup: translated label -> action.
// Collected on the first call for ALL supported languages
// to recognize buttons rendered before the language was changed.
var buttonActions map[string]string

// MenuActionByText returns the action based on the text of the reply button.
// An empty string if the text is not a menu button.
func MenuActionByText(text string) string {
	if buttonActions == nil {
		buildButtonActions()
	}
	return buttonActions[text]
}

func buildButtonActions() {
	m := make(map[string]string, len(buttonKeys)*len(i18n.SupportedLanguages()))
	for _, lang := range i18n.SupportedLanguages() {
		for key, action := range buttonKeys {
			if label := i18n.TByLang(lang, key); label != "" {
				m[label] = action
			}
		}
	}
	buttonActions = m
}

// InitMainMenu builds the reply keyboard tailored to the user’s language.
// webAppURL can be empty — in that case, the “Screener” button is not added.
func InitMainMenu(webAppURL, lang string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{ResizeKeyboard: true}

	lblMarket := i18n.TByLang(lang, "btn_market")
	lblFav := i18n.TByLang(lang, "btn_favorites")
	lblSettings := i18n.TByLang(lang, "btn_settings")

	btnMarket := menu.Text(lblMarket)
	btnFav := menu.Text(lblFav)
	btnSettings := menu.Text(lblSettings)

	if webAppURL == "" {
		menu.Reply(
			menu.Row(btnMarket, btnFav),
			menu.Row(btnSettings),
		)
		return menu
	}

	lblScreener := i18n.TByLang(lang, "btn_screener")
	screenerURL := strings.TrimRight(webAppURL, "/") + "/static/chart.html?mode=market&lang=" + url.QueryEscape(lang)
	btnScreener := menu.WebApp(lblScreener, &tele.WebApp{URL: screenerURL})

	menu.Reply(
		menu.Row(btnMarket, btnFav),
		menu.Row(btnScreener, btnSettings),
	)
	return menu
}