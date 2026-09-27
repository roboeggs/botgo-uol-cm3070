package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"botgo/internal/database"
	"botgo/internal/market"

	"github.com/openai/openai-go"
)

// ProgressStage — a stage in the operation of the AI module.
type ProgressStage string

const (
	StageClassifying ProgressStage = "classifying"
	StageLoadingData ProgressStage = "loading_data"
	StageLoadingTop ProgressStage = "loading_top"
	StageComparing ProgressStage = "comparing"
	StageAnalyzing ProgressStage = "analyzing"
	StageWritingFinal ProgressStage = "writing_final"
)

// ProgressFunc is called at each stage. It may be nil.
type ProgressFunc func(stage ProgressStage, detail string)

// Sentinel errors that the handler can convert into i18n responses.
var (
	// ErrUnknownQuery — the query could not be assigned to any 
	ErrUnknownQuery = errors.New("query is unclear")
	// ErrLLMEmpty — the model returned an empty or garbage response.
	ErrLLMEmpty = errors.New("llm returned empty content")
	// ErrLLMTimeout — the model did not respond within the allotted time.
	ErrLLMTimeout = errors.New("llm timeout")
)

// Analyze — the main input of the AI module.
func (s *Service) Analyze(ctx context.Context, userID int64, lang, prompt string, onProgress ProgressFunc) (string, error) {

	if onProgress == nil {
		onProgress = func(ProgressStage, string) {}
	}

	onProgress(StageClassifying, "")

	classification, err := s.ClassifyQuery(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("classify: %w", err)
	}

	slog.Info("AI classified",
		"user_id", userID,
		"intent", classification.Intent,
		"tickers", classification.Tickers,
		"lang", lang,
		"reason", classification.Reason,
	)

	var out string
	switch classification.Intent {
	case IntentAnalyzeTicker:
		if len(classification.Tickers) == 0 {
			return "", ErrUnknownQuery
		}
		out, err = s.analyzeTicker(ctx, classification.Tickers[0], lang, onProgress)

	case IntentTopML:
		out, err = s.analyzeTop(ctx, lang, onProgress)

	case IntentCompare:
		if len(classification.Tickers) < 2 {
			return "", ErrUnknownQuery
		}
		out, err = s.analyzeCompare(ctx, classification.Tickers, lang, onProgress)

	case IntentConcept:
		out, err = s.answerConcept(ctx, prompt, lang, onProgress)

	default:
		return "", ErrUnknownQuery
	}

	if err != nil {
		return "", err
	}

	if looksLikeEmptyOrJunk(out) {
		slog.Warn("Analyze produced junk output",
			"user_id", userID, "len", len(out))
		return "", ErrLLMEmpty
	}

	return out, nil
}


func (s *Service) analyzeTicker(ctx context.Context, ticker, lang string, onProgress ProgressFunc) (string, error) {

	onProgress(StageLoadingData, ticker)

	row, err := database.GetTicker(ctx, ticker)
	if err != nil {
		return "", fmt.Errorf("get ticker: %w", err)
	}
	if row == nil {
		return "", fmt.Errorf("ticker %s not found", ticker)
	}

	var mktPtr *market.MarketData
	if data, err := market.GetMarketData(ctx, ticker); err == nil {
		mktPtr = &data
	} else {
		slog.Warn("market data unavailable", "ticker", ticker, "err", err)
	}

	payload, err := buildTickerPayload(row, mktPtr)
	if err != nil {
		return "", err
	}

	onProgress(StageAnalyzing, ticker)
	return s.callLLM(ctx, tickerAnalyzePrompt + langInstruction(lang), payload)
}

// langInstruction returns a string instruction about the response language.
func langInstruction(lang string) string {
	switch strings.ToLower(lang) {
	case "ru":
		return "\n\nIMPORTANT: Respond in Russian. Do not use English."
	case "en":
		return "\n\nIMPORTANT: Respond in English."
	default:
		return ""
	}
}


func (s *Service) analyzeTop(ctx context.Context, lang string, onProgress ProgressFunc) (string, error) {

	onProgress(StageLoadingTop, "")

	stats, err := database.GetTopByBreakout(ctx, 5)
	if err != nil {
		return "", fmt.Errorf("top tickers: %w", err)
	}
	if len(stats) == 0 {
		return "", fmt.Errorf("no active tickers")
	}

	top := stats
	if len(top) > 3 {
		top = top[:3]
	}

	payloads := make([]json.RawMessage, 0, len(top))
	for _, st := range top {
		onProgress(StageLoadingData, st.Ticker)

		row, err := database.GetTicker(ctx, st.Ticker)
		if err != nil || row == nil {
			continue
		}
		var mktPtr *market.MarketData
		if data, err := market.GetMarketData(ctx, st.Ticker); err == nil {
			mktPtr = &data
		}
		p, err := buildTickerPayload(row, mktPtr)
		if err != nil {
			continue
		}
		payloads = append(payloads, json.RawMessage(p))
	}

	if len(payloads) == 0 {
		return "", fmt.Errorf("no ticker payloads")
	}

	onProgress(StageAnalyzing, "")
	body, _ := json.Marshal(payloads)
	return s.callLLM(ctx, topMLPrompt+langInstruction(lang), string(body))
}


func (s *Service) analyzeCompare(ctx context.Context, tickers []string, lang string, onProgress ProgressFunc) (string, error) {

	if len(tickers) > 4 {
		tickers = tickers[:4]
	}

	onProgress(StageComparing, "")

	payloads := make([]json.RawMessage, 0, len(tickers))
	for _, ticker := range tickers {
		onProgress(StageLoadingData, ticker)

		row, err := database.GetTicker(ctx, ticker)
		if err != nil || row == nil {
			continue
		}
		var mktPtr *market.MarketData
		if data, err := market.GetMarketData(ctx, ticker); err == nil {
			mktPtr = &data
		}
		p, err := buildTickerPayload(row, mktPtr)
		if err != nil {
			continue
		}
		payloads = append(payloads, json.RawMessage(p))
	}

	if len(payloads) < 2 {
		return "", ErrUnknownQuery
	}

	onProgress(StageAnalyzing, "")
	body, _ := json.Marshal(payloads)
	return s.callLLM(ctx, comparePrompt+langInstruction(lang), string(body))
}



func (s *Service) answerConcept(ctx context.Context, prompt, lang string, onProgress ProgressFunc) (string, error) {

	onProgress(StageAnalyzing, "")
	return s.callLLM(ctx, conceptPrompt+langInstruction(lang), prompt)
}


//  LLM helper — a single call without tool calling
func (s *Service) callLLM(ctx context.Context, systemPrompt, userData string) (string, error) {
	resp, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: OpenAIModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userData),
		},
		MaxCompletionTokens: openai.Int(4000),
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrLLMTimeout
		}
		return "", fmt.Errorf("llm call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm: no choices")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	if content == "" || looksLikeEmptyOrJunk(content) {
		slog.Warn("LLM returned empty/junk content",
			"finish_reason", resp.Choices[0].FinishReason,
			"model", OpenAIModel,
		)
		return "", ErrLLMEmpty
	}

	return content, nil
}


//  Payload builder — a compact JSON for LLM

type levelView struct {
	LevelID int `json:"level_id"`
	Type string `json:"level_type"`
	Center float64 `json:"center"`
	ZoneLow float64 `json:"zone_low"`
	ZoneHigh float64 `json:"zone_high"`
	TouchCount int `json:"touch_count"`
	FirstTouch string `json:"first_touch_time"`
	LastTouch string `json:"last_touch_time"`
	ML *mlView `json:"ml_prediction"`
}

type mlView struct {
	Breakout1h *breakoutView `json:"breakout_1h"`
	Breakout4h *breakoutView `json:"breakout_4h"`
	Breakout24h *breakoutView `json:"breakout_24h"`
}

type breakoutView struct {
	Probability float64 `json:"probability"`
	Threshold float64 `json:"threshold"`
}

type blobView struct {
	Levels []levelView `json:"levels"`
}

// buildTickerPayload builds a compact JSON: only the necessary fields,
// without internal metrics, without raw touch_times.
func buildTickerPayload(row *database.DetectorStateRow, mkt *market.MarketData) (string, error) {
	var blob blobView
	if err := json.Unmarshal(row.FilteredStateBlob, &blob); err != nil {
		return "", fmt.Errorf("parse state blob: %w", err)
	}

	type levelOut struct {
		Type string `json:"type"`
		Center float64 `json:"center"`
		Zone [2]float64 `json:"zone"`
		Touches int `json:"touches"`
		FirstTouch string `json:"first_touch,omitempty"`
		LastTouch string `json:"last_touch,omitempty"`
		Breakout1h *float64 `json:"breakout_1h,omitempty"`
		Breakout4h *float64 `json:"breakout_4h,omitempty"`
		Breakout24h *float64 `json:"breakout_24h,omitempty"`
	}

	levels := make([]levelOut, 0, len(blob.Levels))
	for _, l := range blob.Levels {
		lo := levelOut{
			Type: l.Type,
			Center: l.Center,
			Zone: [2]float64{l.ZoneLow, l.ZoneHigh},
			Touches: l.TouchCount,
			FirstTouch: l.FirstTouch,
			LastTouch: l.LastTouch,
		}
		if l.ML != nil {
			if l.ML.Breakout1h != nil {
				p := l.ML.Breakout1h.Probability
				lo.Breakout1h = &p
			}
			if l.ML.Breakout4h != nil {
				p := l.ML.Breakout4h.Probability
				lo.Breakout4h = &p
			}
			if l.ML.Breakout24h != nil {
				p := l.ML.Breakout24h.Probability
				lo.Breakout24h = &p
			}
		}
		levels = append(levels, lo)
	}

	type out struct {
		Ticker string `json:"ticker"`
		HasActive bool `json:"has_active_levels"`
		LastProcessed *int64 `json:"last_processed,omitempty"`
		Price *float64 `json:"price,omitempty"`
		Change24h *float64 `json:"change_24h,omitempty"`
		High24h *float64 `json:"high_24h,omitempty"`
		Low24h  *float64 `json:"low_24h,omitempty"`
		Volume24h *float64 `json:"volume_24h,omitempty"`
		EMA20 *float64 `json:"ema20,omitempty"`
		RSI14 *float64 `json:"rsi14,omitempty"`
		Levels []levelOut `json:"levels"`
	}

	o := out{
		Ticker: row.Ticker,
		HasActive: row.HasActiveLevels,
		LastProcessed: row.LastProcessedTime,
		Levels: levels,
	}
	if mkt != nil {
		p, c, h, l, v := mkt.Price, mkt.Change24h, mkt.High24h, mkt.Low24h, mkt.Volume24h
		e, r := mkt.EMA20, mkt.RSI14
		o.Price, o.Change24h, o.High24h, o.Low24h, o.Volume24h = &p, &c, &h, &l, &v
		o.EMA20, o.RSI14 = &e, &r
	}

	b, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	return string(b), nil
}


//  Prompts for Luna

const tickerAnalyzePrompt = `You are a crypto market analyst. Analyze the ticker data below.

Write 3-5 concise sentences:
1. Current price vs nearest support/resistance levels
2. RSI(14) and EMA(20) interpretation
3. ML breakout probabilities (1h / 4h / 24h) — which horizon matters most
4. What to watch (key level or signal)

Rules:
- Use only the data provided. Do not invent numbers.
- No disclaimers, no "not financial advice".
- Refer to levels by their numeric price, not by index.
- Plain text only, no markdown headers.

DATA:
`

const topMLPrompt = `You are a crypto market analyst. Below are top tickers selected by an ML model
for high breakout probability.

For each ticker, write 1-2 sentences in the user's language:
- Breakout probability and horizon
- Where price sits relative to nearest S/R
- Whether RSI/EMA confirm or contradict the signal

Rules:
- Use only provided data. No invented numbers.
- No disclaimers.
- Plain text only, no markdown headers.

DATA:
`

const comparePrompt = `You are a crypto market analyst. Compare the tickers below.

Write one short paragraph per ticker (2-3 sentences each), then one closing sentence
comparing them (which has stronger signal, which is riskier).

Rules:
- Use only provided data. No invented numbers.
- No disclaimers.
- Plain text only, no markdown headers.

DATA:
`

const conceptPrompt = `You are a crypto trading educator. Answer the user's question clearly and concisely.

Rules:
- 3-6 sentences, plain language.
- If the question is about a term (RSI, EMA, support/resistance), explain with a simple example.
- Answer in the same language as the question.
- No disclaimers.

Question:
`