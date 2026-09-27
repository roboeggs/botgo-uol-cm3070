package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"botgo/internal/generate"

	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

// stageUpdateInterval — the minimum pause between Edit's of the placeholder.
// Telegram limits edits, 1.5 sec is safe.
const stageUpdateInterval = 1500 * time.Millisecond

func HandleAI(
	aiService *generate.Service,
) func(tele.Context) error {

	return func(c tele.Context) error {

		if c.Sender() == nil {
			return nil
		}

		userID := c.Sender().ID

		prompt := strings.TrimSpace(
			c.Message().Payload,
		)

		if prompt == "" {
			return reply(c, i18n.T(userID, "ai_usage"))
		}

		slog.Info(
			"AI request",
			"user_id", userID,
			"prompt", prompt,
		)

		ctx, cancel := context.WithTimeout(
			context.Background(),
			120*time.Second,
		)
		defer cancel()

		// Placeholder — we will edit it as progress is made.
		placeholder, err := c.Bot().Send(
			c.Chat(),
			i18n.T(userID, "ai_thinking"),
		)
		if err != nil {
			slog.Warn("failed to send AI placeholder",
				"user_id", userID, "err", err)
			placeholder = nil
		}

		// Typing indicator in the background.
		go startTyping(ctx, c)

		// Progress callback with throttle.
		progress := newProgressTracker(c, placeholder, userID)

		lang := i18n.GetLanguage(userID)

		answer, err := aiService.Analyze(
			ctx,
			userID,
			lang,
			prompt,
			progress.Update,
		)

		// Disable typing.
		cancel()

		// Final Edit — replace the placeholder with the response.
		// The progress tracker releases the internal lock so that we can
		// make the final Edit without a race condition.
		progress.Stop()

		if err != nil {
			switch {
			case errors.Is(err, generate.ErrUnknownQuery):
				slog.Info("AI query unclear",
					"user_id", userID, "prompt", prompt)
				return replaceOrReply(c, placeholder,
					i18n.T(userID, "ai_unknown"))

			case errors.Is(err, generate.ErrLLMTimeout):
				slog.Warn("AI request timed out",
					"user_id", userID, "prompt", prompt)
				return replaceOrReply(c, placeholder,
					i18n.T(userID, "ai_timeout"))

			case errors.Is(err, generate.ErrLLMEmpty):
				slog.Warn("AI returned empty answer",
					"user_id", userID, "prompt", prompt)
				return replaceOrReply(c, placeholder,
					i18n.T(userID, "ai_empty"))
			}

			slog.Error("AI request failed",
				"user_id", userID, "error", err)
			return replaceOrReply(c, placeholder,
				i18n.T(userID, "ai_error"))
		}

		slog.Info("AI response generated",
			"user_id", userID, "len", len(answer))

		return replaceOrReply(c, placeholder, answer)
	}
}

//  ProgressTracker — updates the placeholder with throttle.

type progressTracker struct {
	mu          sync.Mutex
	bot         tele.API
	placeholder *tele.Message
	userID      int64
	lastEdit    time.Time
	lastStage   generate.ProgressStage
	lastDetail  string
	stopped     bool
}

func newProgressTracker(
	c tele.Context,
	placeholder *tele.Message,
	userID int64,
) *progressTracker {
	return &progressTracker{
		bot:         c.Bot(),
		placeholder: placeholder,
		userID:      userID,
	}
}

func (p *progressTracker) Update(stage generate.ProgressStage, detail string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped || p.placeholder == nil {
		return
	}

	// Skip duplicates
	if stage == p.lastStage && detail == p.lastDetail {
		return
	}
	p.lastStage = stage
	p.lastDetail = detail

	// Throttle: no more than once per stageUpdateInterval.
	if time.Since(p.lastEdit) < stageUpdateInterval {
		return
	}
	p.lastEdit = time.Now()

	text := renderStage(p.userID, stage, detail)
	if _, err := p.bot.Edit(p.placeholder, text); err != nil {
		slog.Debug("progress edit failed",
			"user_id", p.userID, "stage", stage, "err", err)
	}
}

func (p *progressTracker) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
}

// renderStage collects the multi-line progress text.

func renderStage(
	userID int64,
	stage generate.ProgressStage,
	detail string,
) string {
	var sb strings.Builder
	sb.WriteString(i18n.T(userID, "ai_thinking"))

	line := stageLine(userID, stage, detail)
	if line != "" {
		sb.WriteString("\n\n")
		sb.WriteString(line)
	}
	return sb.String()
}

func stageLine(
	userID int64,
	stage generate.ProgressStage,
	detail string,
) string {
	switch stage {
	case generate.StageClassifying:
		return "→ " + i18n.T(userID, "ai_stage_classify")

	case generate.StageLoadingData:
		if detail != "" {
			return "→ " + i18n.T(userID, "ai_stage_loading_ticker",
				map[string]interface{}{"Ticker": detail})
		}
		return "→ " + i18n.T(userID, "ai_stage_loading")

	case generate.StageLoadingTop:
		return "→ " + i18n.T(userID, "ai_stage_loading_top")

	case generate.StageComparing:
		return "→ " + i18n.T(userID, "ai_stage_comparing")

	case generate.StageAnalyzing:
		if detail != "" {
			return "→ " + i18n.T(userID, "ai_stage_analyzing_ticker",
				map[string]interface{}{"Ticker": detail})
		}
		return "→ " + i18n.T(userID, "ai_stage_analyzing")

	default:
		return ""
	}
}


func startTyping(ctx context.Context, c tele.Context) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()

	_ = c.Notify(tele.Typing)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = c.Notify(tele.Typing)
		}
	}
}

func replaceOrReply(
	c tele.Context,
	placeholder *tele.Message,
	text string,
) error {
	if placeholder == nil {
		return reply(c, text)
	}
	_, err := c.Bot().Edit(placeholder, text)
	if err != nil {
		return reply(c, text)
	}
	return nil
}

