package handlers

import (
	"unicode/utf8"

	"botgo/internal/telegram"

	tele "gopkg.in/telebot.v4"
)

// longMessageThreshold — runes, not bytes. 3500 provides a margin
// up to the Telegram limit of 4096 UTF-16 units
const longMessageThreshold = 3500

var sender *telegram.Sender

// SetSender saves the Sender for use in the reply helper.
func SetSender(s *telegram.Sender) {
	sender = s
}

// webAppBaseURL stores the base WebApp URL set during Register.
var webAppBaseURL string

func SetWebAppURL(u string) { webAppBaseURL = u }

func webAppURLFromContext(_ tele.Context) string { return webAppBaseURL }

// reply is the unified point for sending text to the chat.
//
// Short messages are sent synchronously via c.Send (instantly,
// with any options — keyboard, ParseMode, etc.).
//
// Long ones — via Sender.Enqueue: broken into chunks, sent
// with pauses, retried on 429. Returns nil immediately, sending goes
// in the background.
func reply(c tele.Context, text string, opts ...interface{}) error {
	if sender != nil && utf8.RuneCountInString(text) > longMessageThreshold {
		sender.Enqueue(c.Chat().ID, text)
		return nil
	}
	return c.Send(text, opts...)
}