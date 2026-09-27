package handlers

import (
	"strings"
	"sync"
	"time"

	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

const (
	// maxMessageWords — the maximum number of words in an incoming message.
	maxMessageWords = 2000
	// rateCleanupFactor — how long we keep entries in the window before cleaning up the map.
	rateCleanupFactor = 10
)

// rateLimiter — an in-memory per-user limiter with lazy cleanup.
type rateLimiter struct {
	mu      sync.Mutex
	seen    map[int64]time.Time
	window  time.Duration
	cleaned time.Time
}

func newRateLimiter(window time.Duration) *rateLimiter {
	return &rateLimiter{
		seen:   make(map[int64]time.Time),
		window: window,
	}
}

func (r *rateLimiter) allow(userID int64) bool {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// lazy cleanup of outdated entries
	if now.Sub(r.cleaned) > r.window*rateCleanupFactor {
		for id, ts := range r.seen {
			if now.Sub(ts) > r.window {
				delete(r.seen, id)
			}
		}
		r.cleaned = now
	}

	if last, ok := r.seen[userID]; ok && now.Sub(last) < r.window {
		return false
	}
	r.seen[userID] = now
	return true
}

// Separate limiters for semantic zones — so that AI and regular commands
// don’t eat up slots from each other.
var (
	aiRateLimiter      = newRateLimiter(time.Minute)        // /ai: 1/min
	generalRateLimiter = newRateLimiter(3 * time.Second)    // others: 1/3 sec
)

// AdminOnly — middleware that allows only administrators to pass through.
func AdminOnly(adminID int64) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Sender().ID != adminID {
				return reply(c, i18n.T(c.Sender().ID, "access_denied"))
			}
			return next(c)
		}
	}
}

// RateLimit — middleware on top of a specific limiter.
func RateLimit(rl *rateLimiter) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if !rl.allow(c.Sender().ID) {
				return reply(c, i18n.T(c.Sender().ID, "rate_limit_exceeded"))
			}
			return next(c)
		}
	}
}

// MaxWords — limits the length of the incoming text to N words.
func MaxWords(limit int) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			text := c.Text()
			if text != "" && len(strings.Fields(text)) > limit {
				return reply(c, i18n.T(c.Sender().ID, "message_too_long", map[string]interface{}{
					"Limit": limit,
				}))
			}
			return next(c)
		}
	}
}