package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openai/openai-go"
)

type Intent string

const (
	IntentAnalyzeTicker Intent = "analyze_ticker"
	IntentTopML         Intent = "top_ml"
	IntentCompare       Intent = "compare"
	IntentConcept       Intent = "concept"
	IntentUnknown       Intent = "unknown"
)

// Classification — the result of parsing the user's query.
type Classification struct {
	Intent Intent `json:"intent"`
	Tickers []string `json:"tickers"`
	Reason string `json:"reason"`
}

const classifierPrompt = `You are a query classifier for a crypto trading bot.
Return ONLY valid JSON matching this schema:

{
  "intent": "analyze_ticker" | "top_ml" | "compare" | "concept" | "unknown",
  "tickers": ["BTCUSDT"],
  "reason": "short explanation"
}

Intent rules:
- analyze_ticker: user asks about ONE specific ticker
  (e.g. "расскажи про BTC", "what about SUI", "что если BTC пробьёт 65000")
- compare: user asks to compare or discuss TWO OR MORE tickers
- top_ml: user asks what is interesting / hot / worth watching, no specific ticker
- concept: general question about trading or crypto terms (RSI, EMA, support/resistance)
- unknown: greeting, chit-chat, or anything that doesn't fit above

Ticker rules:
- Always UPPERCASE, always with USDT suffix. "BTC" / "btc" → "BTCUSDT".
- top_ml, concept, unknown → tickers = []
- analyze_ticker → exactly one ticker
- compare → two or more tickers

Return ONLY JSON, no markdown fence, no explanation.
`

// ClassifyQuery parses the query into Classification. One Luna call.
func (s *Service) ClassifyQuery(ctx context.Context, prompt string) (*Classification, error) {
	resp, err := s.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: OpenAIModel,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(classifierPrompt),
			openai.UserMessage(prompt),
		},
		MaxCompletionTokens: openai.Int(1500),
	})
	if err != nil {
		return nil, fmt.Errorf("classifier call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("classifier: no choices")
	}

	content := stripJSONFence(resp.Choices[0].Message.Content)

	var c Classification
	if err := json.Unmarshal([]byte(content), &c); err != nil {
		slog.Warn("classifier returned invalid JSON", "raw", content, "err", err)
		return &Classification{Intent: IntentUnknown, Reason: "parse error"}, nil
	}

	// We normalize tickers — in case Luna returned "btc" without USDT.
	c.Tickers = normalizeTickers(c.Tickers)

	return &c, nil
}

func normalizeTickers(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]bool)
	for _, t := range in {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if !strings.HasSuffix(t, "USDT") {
			t += "USDT"
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}