package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"botgo/internal/database"
	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)


// HandleFavorites — shows the list of favorites + inline-WebApp with a fresh fav=.
func HandleFavorites(c tele.Context) error {
	userID := c.Sender().ID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	favs, err := database.GetFavorites(ctx, userID)
	if err != nil {
		slog.Error("GetFavorites failed", "user_id", userID, "error", err)
		return reply(c, i18n.T(userID, "db_error"))
	}

	if len(favs) == 0 {
		return reply(c, i18n.T(userID, "favorites_empty"))
	}

	var sb strings.Builder
	sb.WriteString(i18n.T(userID, "favorites_header", map[string]interface{}{
		"Count": len(favs),
	}))
	sb.WriteString("\n\n")
	for _, f := range favs {
		sb.WriteString(fmt.Sprintf("• <code>%s</code>\n", f))
	}

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Inline-WebApp with a personal fav=...
	if webAppBaseURL != "" {
		short, err := database.GetFavoritesShort(ctx, userID)
		if err == nil && short != "" {
			lang := i18n.GetLanguage(userID)
			params := url.Values{}
			params.Set("fav", short)
			params.Set("lang", lang)
			params.Set("theme", "auto")
			appURL := strings.TrimRight(webAppBaseURL, "/") + "/static/app.html?" + params.Encode()
			rows = append(rows, menu.Row(
				menu.WebApp(i18n.T(userID, "favorites_open_btn"), &tele.WebApp{URL: appURL}),
			))
		}
	}

	rows = append(rows, menu.Row(
		menu.Data(i18n.T(userID, "favorites_manage_btn"), "fav_manage"),
	))
	menu.Inline(rows...)

	return reply(c, sb.String(), menu)
}


// handleMarketOpen — generates an inline-WebApp with a personal fav= and lang=.
func handleMarketOpen(c tele.Context) error {
	userID := c.Sender().ID

	if webAppBaseURL == "" {
		return reply(c, i18n.T(userID, "webapp_not_configured"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	short, _ := database.GetFavoritesShort(ctx, userID)
	lang := i18n.GetLanguage(userID)

	params := url.Values{}
	if short != "" {
		params.Set("fav", short)
	}
	params.Set("lang", lang)
	params.Set("theme", "auto")

	appURL := strings.TrimRight(webAppBaseURL, "/") + "/static/app.html?" + params.Encode()

	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.WebApp(i18n.T(userID, "market_open_btn"), &tele.WebApp{URL: appURL}),
	))

	return reply(c, i18n.T(userID, "market_hint"), menu)
}


// HandleFavManage — the “Manage” button.
func HandleFavManage(c tele.Context) error {
	defer c.Respond(&tele.CallbackResponse{})
	userID := c.Sender().ID

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	favs, err := database.GetFavorites(ctx, userID)
	if err != nil {
		slog.Error("GetFavorites failed", "user_id", userID, "error", err)
		return c.Edit(i18n.T(userID, "db_error"))
	}
	if len(favs) == 0 {
		return c.Edit(i18n.T(userID, "favorites_empty"))
	}

	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, f := range favs {
		rows = append(rows, menu.Row(
			menu.Data(f+"  ✖️", "fdel", f),
		))
	}
	rows = append(rows, menu.Row(
		menu.Data(i18n.T(userID, "favorites_done_btn"), "fav_done"),
	))
	menu.Inline(rows...)

	return c.Edit(i18n.T(userID, "favorites_manage_title"), menu)
}

// HandleFavDelete — callback from “Manage”: delete and update the list.
func HandleFavDelete(c tele.Context) error {
	defer c.Respond(&tele.CallbackResponse{})
	userID := c.Sender().ID

	ticker := strings.TrimSpace(c.Callback().Data)
	if ticker == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := database.RemoveFavorite(ctx, userID, ticker); err != nil {
		slog.Error("RemoveFavorite failed", "user_id", userID, "ticker", ticker, "error", err)
	}

	// Show the updated management list
	return HandleFavManage(c)
}

// HandleFavDone — close the management menu, show the regular list.
func HandleFavDone(c tele.Context) error {
	defer c.Respond(&tele.CallbackResponse{})

	userID := c.Sender().ID
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	favs, err := database.GetFavorites(ctx, userID)
	if err != nil {
		return c.Edit(i18n.T(userID, "db_error"))
	}
	if len(favs) == 0 {
		return c.Edit(i18n.T(userID, "favorites_empty"))
	}

	var sb strings.Builder
	sb.WriteString(i18n.T(userID, "favorites_header", map[string]interface{}{
		"Count": len(favs),
	}))
	sb.WriteString("\n\n")
	for _, f := range favs {
		sb.WriteString(fmt.Sprintf("• <code>%s</code>\n", f))
	}

	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data(i18n.T(userID, "favorites_manage_btn"), "fav_manage"),
	))
	return c.Edit(sb.String(), menu)
}


// HandleFavToggle — adds/removes from the ticker card.
// add=true — add, add=false — remove. After that, it updates the card.
func HandleFavToggle(add bool) tele.HandlerFunc {
	return func(c tele.Context) error {
		defer c.Respond(&tele.CallbackResponse{})
		userID := c.Sender().ID

		ticker := strings.TrimSpace(c.Callback().Data)
		if ticker == "" {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		if add {
			err = database.AddFavorite(ctx, userID, ticker)
		} else {
			err = database.RemoveFavorite(ctx, userID, ticker)
		}

		switch {
		case errors.Is(err, database.ErrFavoriteLimitReached):
			return c.Respond(&tele.CallbackResponse{
				Text: i18n.T(userID, "favorites_limit_reached"),
			})
		case errors.Is(err, database.ErrInvalidTicker):
			return c.Respond(&tele.CallbackResponse{
				Text: i18n.T(userID, "favorites_invalid"),
			})
		case err != nil:
			slog.Error("fav toggle failed",
				"user_id", userID, "ticker", ticker, "add", add, "error", err)
			return c.Respond(&tele.CallbackResponse{
				Text: i18n.T(userID, "db_error"),
			})
		}

		// We rebuild the card with the current state of the favorite.
		text, menu, err := buildTickerCard(userID, ticker, webAppBaseURL)
		if err != nil || menu == nil {
			return nil
		}
		return c.Edit(text, menu)
	}
}