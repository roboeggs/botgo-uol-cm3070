package handlers

import (
	"context"
	"encoding/json"
    "errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"net/url"
	"math"

	"botgo/internal/database"
	"botgo/internal/i18n"

	tele "gopkg.in/telebot.v4"
)

// Level — one active level from state_blob
type Level struct {
	LevelID        int      `json:"level_id"`
	Center         float64  `json:"center"`
	ZoneLow        float64  `json:"zone_low"`
	ZoneHigh       float64  `json:"zone_high"`
	LevelType      string   `json:"level_type"`      // "support" | "resistance"
	TouchCount     int      `json:"touch_count"`
	TouchTimes     []string `json:"touch_times"`
	FirstTouchTime string   `json:"first_touch_time"`
	LastTouchTime  string   `json:"last_touch_time"`
}

// Sentinel errors for buildTickerCard
var (
	ErrTickerNotFound = errors.New("ticker not found")
	ErrTickerDB       = errors.New("ticker db error")
	ErrTickerParse    = errors.New("ticker parse error")
)

// StateBlob is a structure for parsing FilteredStateBlob
type StateBlob struct {
	Levels []Level `json:"levels"`
}


// formatPrice adaptively formats the price of a level depending
// on the magnitude: BTC → 2 decimal places, SUI → 4, SHIB → 8, etc.
// Rule: no fewer than 2 and no more than 10 decimal places,
// we focus on 4 significant digits.
func formatPrice(p float64) string {
	if p == 0 {
		return "0"
	}
	abs := math.Abs(p)

	// We determine the order of the number 
	var order int
	switch {
	case abs >= 1000:
		order = 2
	case abs >= 100:
		order = 3
	case abs >= 10:
		order = 4
	case abs >= 1:
		order = 4
	case abs >= 0.1:
		order = 5
	case abs >= 0.01:
		order = 6
	case abs >= 0.001:
		order = 7
	case abs >= 0.0001:
		order = 8
	default:
		order = 10
	}

	return fmt.Sprintf("%.*f", order, p)
}

func formatTouchTime(raw string) string {
	if raw == "" {
		return "—"
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, raw); err == nil {
			return t.UTC().Format("2006-01-02 15:04")
		}
	}
	return raw
}

// buildTickerCard generates the card text and inline menu for the ticker.
// It doesn’t send a message — that’s done by the calling handler.
func buildTickerCard(userID int64, ticker string, webAppURL string) (string, *tele.ReplyMarkup, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stateRow, err := database.GetTicker(ctx, ticker)
	if err != nil {
		slog.Error("Database error fetching state", "ticker", ticker, "error", err)
		return "", nil, ErrTickerDB
	}
	if stateRow == nil {
		slog.Warn("Ticker not found", "ticker", ticker)
		return "", nil, ErrTickerNotFound
	}

	var state StateBlob
	if err := json.Unmarshal(stateRow.FilteredStateBlob, &state); err != nil {
		slog.Error("Failed to parse filtered state blob", "ticker", ticker, "error", err)
		return "", nil, ErrTickerParse
	}

	savedAtStr := "N/A"
	if stateRow.LastProcessedTime != nil && *stateRow.LastProcessedTime > 0 {
		savedAtStr = time.Unix(*stateRow.LastProcessedTime, 0).
			UTC().Format("2006-01-02 15:04")
	} else if stateRow.SavedAt != nil {
		// fallback: if for some reason there’s no line in processing_log
		savedAtStr = stateRow.SavedAt.UTC().Format("2006-01-02 15:04")
	}

	statusIcon := "❌"
	if stateRow.HasActiveLevels {
		statusIcon = "✅"
	}

	var sb strings.Builder
	sb.WriteString(i18n.T(userID, "db_header", map[string]interface{}{
		"Ticker":      stateRow.Ticker,
		"ActiveCount": len(state.Levels),
		"Status":      statusIcon,
		"SavedAt":     savedAtStr,
	}))

	if len(state.Levels) > 0 {
		sb.WriteString("\n")
		sb.WriteString(i18n.T(userID, "db_levels_title"))
		sb.WriteString("\n")

		for i, lvl := range state.Levels {
			levelTypeKey := "db_level_type_support"
			if lvl.LevelType == "resistance" {
				levelTypeKey = "db_level_type_resistance"
			}
			levelType := i18n.T(userID, levelTypeKey)

			humanTimes := make([]string, 0, len(lvl.TouchTimes))
			for _, tt := range lvl.TouchTimes {
				humanTimes = append(humanTimes, formatTouchTime(tt))
			}
			touchesBlock := "\n    " + strings.Join(humanTimes, "\n    ")

			rangeStr := formatPrice(lvl.ZoneLow) + " – " + formatPrice(lvl.ZoneHigh)
			if lvl.ZoneLow == lvl.ZoneHigh {
				rangeStr = formatPrice(lvl.ZoneLow)
			}

			sb.WriteString(fmt.Sprintf(
				"\n%s\n%s\n%s\n%s\n",
				i18n.T(userID, "db_level_number", map[string]interface{}{
					"Number": i + 1, "Type": levelType,
				}),
				i18n.T(userID, "db_level_price", map[string]interface{}{
					"Price": formatPrice(lvl.Center),
				}),
				i18n.T(userID, "db_level_range", map[string]interface{}{
					"Range": rangeStr,
				}),
				i18n.T(userID, "db_level_strength", map[string]interface{}{
					"Count":        lvl.TouchCount,
					"FirstTouchAt": formatTouchTime(lvl.FirstTouchTime),
					"LastTouchAt":  formatTouchTime(lvl.LastTouchTime),
				}),
			))

			sb.WriteString(i18n.T(userID, "db_level_touches", map[string]interface{}{
				"Touches": touchesBlock,
			}))
			sb.WriteString("\n")
		}
	}

	text := sb.String()

	// Button menu under the card
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row

	// Determine the favorite state once
	ctxFav, cancelFav := context.WithTimeout(context.Background(), 3*time.Second)
	isFav, _ := database.IsFavorite(ctxFav, userID, ticker)
	cancelFav()

	// The chart is shown only if there’s a WebApp URL
	if webAppURL != "" {
		lang := i18n.GetLanguage(userID)
		params := url.Values{}
		params.Set("mode", "chart")
		params.Set("ticker", ticker)
		params.Set("lang", lang)
		fullURL := strings.TrimRight(webAppURL, "/") + "/static/chart.html?" + params.Encode()

		btnChart := menu.WebApp(
			i18n.T(userID, "db_open_chart_btn", map[string]interface{}{"Ticker": ticker}),
			&tele.WebApp{URL: fullURL},
		)
		rows = append(rows, menu.Row(btnChart))
	}

	// The favorite button
	if isFav {
		rows = append(rows, menu.Row(
			menu.Data(i18n.T(userID, "fav_btn_remove"), "fcard_del", ticker),
		))
	} else {
		rows = append(rows, menu.Row(
			menu.Data(i18n.T(userID, "fav_btn_add"), "fcard_add", ticker),
		))
	}

	if len(rows) == 0 {
		return text, nil, nil
	}
	menu.Inline(rows...)

	return text, menu, nil
}

func HandleDB(webAppURL string) tele.HandlerFunc {
	return func(c tele.Context) error {
		userID := c.Sender().ID

		ticker := strings.TrimSpace(strings.ToUpper(c.Message().Payload))
		if ticker == "" {
			return reply(c, i18n.T(userID, "db_usage"))
		}

		text, menu, err := buildTickerCard(userID, ticker, webAppURL)
		switch {
		case errors.Is(err, ErrTickerNotFound):
			return reply(c, i18n.T(userID, "db_ticker_not_found", map[string]interface{}{
				"Ticker": ticker,
			}))
		case errors.Is(err, ErrTickerDB), errors.Is(err, ErrTickerParse):
			return reply(c, i18n.T(userID, "db_error"))
		case err != nil:
			slog.Error("buildTickerCard unexpected error", "error", err)
			return reply(c, i18n.T(userID, "db_error"))
		}

		if menu == nil {
			return reply(c, text)
		}
		_, sendErr := c.Bot().Send(c.Chat(), text, menu)
		return sendErr
	}
}

func HandleDBTickers(webAppURL string) tele.HandlerFunc {
    return func(c tele.Context) error {
        userID := c.Sender().ID

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        tickers, err := database.GetTickerActiveLevels(ctx)
        if err != nil {
            slog.Error("Database error fetching active tickers", "error", err)
            return reply(c, i18n.T(userID, "db_tickers_error"))
        }

        if len(tickers) == 0 {
            return reply(c, i18n.T(userID, "db_tickers_empty"))
        }

        var listBuilder strings.Builder
        listBuilder.WriteString(i18n.T(userID, "db_tickers_header"))
        listBuilder.WriteString("\n\n")
        for _, t := range tickers {
            listBuilder.WriteString(fmt.Sprintf("• <code>%s</code>\n", t))
        }
        listBuilder.WriteString("\n")
        listBuilder.WriteString(i18n.T(userID, "db_tickers_total", map[string]interface{}{
            "Count": len(tickers),
        }))

        // If no Web App URL is specified — just the text
        if webAppURL == "" {
            return reply(c, listBuilder.String())
        }

        // We determine the user's language from our own database
        lang := i18n.GetLanguage(userID)

        // We assemble the URL: /static/chart.html?tickers=...&lang=...
        params := url.Values{}
        params.Set("tickers", strings.Join(tickers, ","))
        params.Set("lang", lang)

        fullURL := strings.TrimRight(webAppURL, "/") + "/static/chart.html?" + params.Encode()
        // fullURL := strings.TrimRight(webAppURL, "/") + "/chart.html?" + params.Encode()


        menu := &tele.ReplyMarkup{}
        btn := menu.WebApp("📈 Market Data", &tele.WebApp{URL: fullURL})
        menu.Inline(menu.Row(btn))

        _, err = c.Bot().Send(c.Chat(), listBuilder.String(), menu)
        return err
    }
}