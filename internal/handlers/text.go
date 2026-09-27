package handlers

import (
	"log/slog"
	"regexp"
	"strings"

	"botgo/internal/database"
	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)
// tickerRe — a ticker without a suffix and with it, 2–15 characters.

var tickerRe = regexp.MustCompile(`^[A-Za-z0-9]{2,15}(USDT)?$`)

func HandleText(c tele.Context) error {
	userID := c.Sender().ID

	lang, err := database.UpsertUser(c)
	if err != nil {
		slog.Warn("UpsertUser in OnText failed", "user_id", userID, "error", err)
	}
	i18n.SetLanguageCacheOnly(userID, lang)

	text := strings.TrimSpace(c.Text())

	// Reply buttons — we recognize them by the label in any language.
	switch MenuActionByText(text) {
	case actMarket:
		return handleMarketOpen(c)
	case actFav:
		return HandleFavorites(c)
	case actSettings:
		return HandleSettings(c)
	}

	// Bare ticker — a single word, matches the regex
	if isLikelyTicker(text) {
		return handleTickerCardByText(c, text)
	}

	slog.Debug("Received text message",
		"user_id", userID,
		"username", c.Sender().Username,
		"text", text,
	)
	msg := i18n.T(userID, "echo", map[string]interface{}{"Text": text})
	return reply(c, msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
}

func isLikelyTicker(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	return tickerRe.MatchString(s)
}

// handleTickerCardByText — normalizes the ticker and calls the common builder.
func handleTickerCardByText(c tele.Context, raw string) error {
	userID := c.Sender().ID
	ticker := strings.ToUpper(strings.TrimSpace(raw))
	if !strings.HasSuffix(ticker, "USDT") {
		ticker += "USDT"
	}

	text, menu, err := buildTickerCard(userID, ticker, webAppURLFromContext(c))
	if err != nil {
		return reply(c, i18n.T(userID, "db_ticker_not_found", map[string]interface{}{
			"Ticker": ticker,
		}))
	}
	if menu == nil {
		return reply(c, text)
	}
	_, sendErr := c.Bot().Send(c.Chat(), text, menu)
	return sendErr
}