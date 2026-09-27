package handlers

import (
	"context"
	"errors"
	"log/slog"

	"botgo/internal/generate"
	"botgo/internal/telegram"


	tele "gopkg.in/telebot.v4"
)

func Register(b *tele.Bot, adminID int64, aiService *generate.Service, webAppURL string) {
	s := telegram.NewSender(
		func(ctx context.Context, chatID int64, text string) error {
			_, err := b.Send(tele.ChatID(chatID), text)
			return err
		},
		func(err error) (int, bool) {
			var floodErr *tele.FloodError
			if errors.As(err, &floodErr) && floodErr.RetryAfter > 0 {
				return floodErr.RetryAfter, true
			}
			return 0, false
		},
	)
	SetSender(s)

	SetWebAppURL(webAppURL)


	// adminMiddleware := AdminOnly(adminID)

	aiGuard := func(h tele.HandlerFunc) tele.HandlerFunc {
		// First, length (cheap, doesn’t burn a slot), then rate limit.
		return MaxWords(maxMessageWords)(RateLimit(aiRateLimiter)(h))
	}
	generalGuard := func(h tele.HandlerFunc) tele.HandlerFunc {
		return MaxWords(maxMessageWords)(RateLimit(generalRateLimiter)(h))
	}

	slog.Info("Registering handlers...")

	b.Handle("/start", Handlestart)
	b.Handle(&BtnRu, HandleLanguageRu)
	b.Handle(&BtnEn, HandleLanguageEn)

	b.Handle("/help", generalGuard(HandleHelp))
	b.Handle(tele.OnText, generalGuard(HandleText))
	b.Handle("/db", generalGuard(HandleDB(webAppURL)))
	b.Handle("/db_tickers", generalGuard(HandleDBTickers(webAppURL)))

	// Slash-duplicates reply-buttons — the same logic as for buttons.
	b.Handle("/market",    generalGuard(HandleMenuAction(actMarket)))
	b.Handle("/favorites", generalGuard(HandleMenuAction(actFav)))
	b.Handle("/screener",  generalGuard(HandleMenuAction(actScreener)))
	b.Handle("/settings",  HandleSettings) // navigation, without rate limit


	// Favorites — callbacks
	b.Handle(&tele.Btn{Unique: "fcard_add"}, generalGuard(HandleFavToggle(true)))
	b.Handle(&tele.Btn{Unique: "fcard_del"}, generalGuard(HandleFavToggle(false)))
	b.Handle(&tele.Btn{Unique: "fdel"}, generalGuard(HandleFavDelete))
	b.Handle(&tele.Btn{Unique: "fav_manage"}, generalGuard(HandleFavManage))
	b.Handle(&tele.Btn{Unique: "fav_done"}, generalGuard(HandleFavDone))

	// AI — strict limit of 1/min.
	b.Handle("/ai", aiGuard(HandleAI(aiService)))

	b.Handle("/resetkb", generalGuard(HandleResetKeyboard))

	slog.Info("Handlers registered successfully.")
}