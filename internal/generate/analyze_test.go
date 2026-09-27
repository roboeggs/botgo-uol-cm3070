package generate

import (
	"encoding/json"
	"strings"
	"testing"

	"botgo/internal/database"
	"botgo/internal/market"

	"github.com/openai/openai-go"
)

// ============================================================
//  looksLikeEmptyOrJunk
// ============================================================

func TestLooksLikeEmptyOrJunk(t *testing.T) {
	cases := []struct {
		name string
		in string
		want bool
	}{
		{"empty", "", true},
		{"spaces", "   \n\t  ", true},
		{"only digits", "12345", true},
		{"only punctuation", "!@#$%^", true},
		{"normal text en", "BTC looks bullish", false},
		{"normal text ru", "Биткоин выглядит бычьим", false},
		{"mixed digits+letter", "123 abc", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeEmptyOrJunk(tc.in); got != tc.want {
				t.Fatalf("looksLikeEmptyOrJunk(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ============================================================
//  trimHistory
// ============================================================

func TestTrimHistory(t *testing.T) {
	sys := openai.SystemMessage("S")
	mk := func(n int) []openai.ChatCompletionMessageParamUnion {
		msgs := []openai.ChatCompletionMessageParamUnion{sys}
		for i := 0; i < n; i++ {
			msgs = append(msgs, openai.UserMessage("u"))
			msgs = append(msgs, openai.AssistantMessage("a"))
		}
		return msgs
	}

	t.Run("short history untouched", func(t *testing.T) {
		in := mk(3) // 1 sys + 6 = 7 msgs
		got := trimHistory(in)
		if len(got) != len(in) {
			t.Fatalf("len got %d want %d", len(got), len(in))
		}
	})

	t.Run("long history trimmed", func(t *testing.T) {
		in := mk(maxHistoryTurns + 5) // far above the limit
		got := trimHistory(in)
		// After trimming: 1 sys + 10 turns * 2 = 21
		wantLen := 1 + maxHistoryTurns*2
		if len(got) != wantLen {
			t.Fatalf("len got %d want %d", len(got), wantLen)
		}
		// First message is always the system one.
		if got[0].OfSystem == nil {
			t.Fatalf("first message is not system")
		}
		// After system comes a user message (turn boundary).
		if got[1].OfUser == nil {
			t.Fatalf("second message is not user")
		}
	})

	t.Run("tiny history", func(t *testing.T) {
		in := []openai.ChatCompletionMessageParamUnion{openai.SystemMessage("S")}
		got := trimHistory(in)
		if len(got) != 1 {
			t.Fatalf("single system was modified: len=%d", len(got))
		}
	})
}

// ============================================================
//  normalizeTickers / stripJSONFence  (classifier.go)
// ============================================================

func TestNormalizeTickers(t *testing.T) {
	in := []string{"btc", " ETH ", "BTCUSDT", "", "SUIusdt", "BTC"}
	got := normalizeTickers(in)
	want := []string{"BTCUSDT", "ETHUSDT", "SUIUSDT"}
	if len(got) != len(want) {
		t.Fatalf("len got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("at %d got %s want %s", i, got[i], want[i])
		}
	}
}

func TestStripJSONFence(t *testing.T) {
	cases := map[string]string{
		"```json\n{\"a\":1}\n```": "{\"a\":1}",
		"```\n{\"a\":1}\n```":     "{\"a\":1}",
		"{\"a\":1}":               "{\"a\":1}",
		"  {\"a\":1}  ":           "{\"a\":1}",
	}
	for in, want := range cases {
		if got := stripJSONFence(in); got != want {
			t.Fatalf("stripJSONFence(%q) = %q, want %q", in, got, want)
		}
	}
}

// ============================================================
//  activeTickersResponse / filterAvailableTickers
// ============================================================

func TestFilterAvailableTickers(t *testing.T) {
	in5 := []string{"A", "B", "C", "D", "E"}
	out := filterAvailableTickers(in5)
	if len(out) != maxActiveTickers {
		t.Fatalf("got len %d want %d", len(out), maxActiveTickers)
	}

	in2 := []string{"A", "B"}
	out2 := filterAvailableTickers(in2)
	if len(out2) != 2 {
		t.Fatalf("short slice should be untouched, got %d", len(out2))
	}
}

func TestActiveTickersResponse(t *testing.T) {
	in := []string{"A", "B", "C", "D", "E"}
	resp := activeTickersResponse(in, false)

	if resp["count"] != maxActiveTickers {
		t.Fatalf("count got %v want %d", resp["count"], maxActiveTickers)
	}
	if resp["total_count"] != len(in) {
		t.Fatalf("total_count got %v want %d", resp["total_count"], len(in))
	}
	if resp["truncated"] != true {
		t.Fatalf("truncated got %v want true", resp["truncated"])
	}
	if resp["cached"] != false {
		t.Fatalf("cached got %v want false", resp["cached"])
	}

	// Small selection — not truncated.
	resp2 := activeTickersResponse([]string{"X"}, true)
	if resp2["truncated"] != false {
		t.Fatalf("truncated got %v want false", resp2["truncated"])
	}
	if resp2["cached"] != true {
		t.Fatalf("cached got %v want true", resp2["cached"])
	}
}

// ============================================================
//  formatMarketData
// ============================================================

func TestFormatMarketData(t *testing.T) {
	md := market.MarketData{
		Ticker: "BTCUSDT",
		Price:  63000,
		EMA20:  62500,
		RSI14:  55,
	}
	fresh := formatMarketData(md, false)
	cached := formatMarketData(md, true)

	// Key sets must match.
	if len(fresh) != len(cached) {
		t.Fatalf("key count differs: fresh=%d cached=%d", len(fresh), len(cached))
	}
	for k := range fresh {
		if _, ok := cached[k]; !ok {
			t.Fatalf("key %q missing in cached response", k)
		}
	}
	if fresh["cached"] != false {
		t.Fatalf("fresh.cached got %v want false", fresh["cached"])
	}
	if cached["cached"] != true {
		t.Fatalf("cached.cached got %v want true", cached["cached"])
	}
}

// ============================================================
//  buildTickerPayload
// ============================================================

func TestBuildTickerPayload(t *testing.T) {
	blob := `{
		"levels": [
			{
				"level_id": 1,
				"level_type": "support",
				"center": 62000,
				"zone_low": 61900,
				"zone_high": 62100,
				"touch_count": 3,
				"first_touch_time": "2026-09-20T10:00:00Z",
				"last_touch_time": "2026-09-25T15:30:00Z",
				"ml_prediction": {
					"breakout_1h": {"probability": 0.42, "threshold": 0.5},
					"breakout_24h": {"probability": 0.71, "threshold": 0.6}
				}
			}
		]
	}`

	row := &database.DetectorStateRow{
		Ticker:            "BTCUSDT",
		HasActiveLevels:   true,
		FilteredStateBlob: []byte(blob),
	}

	mkt := &market.MarketData{
		Ticker: "BTCUSDT",
		Price:  63000,
		EMA20:  62500,
		RSI14:  58,
	}

	out, err := buildTickerPayload(row, mkt)
	if err != nil {
		t.Fatalf("buildTickerPayload: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	// Verify key fields.
	if parsed["ticker"] != "BTCUSDT" {
		t.Fatalf("ticker got %v", parsed["ticker"])
	}
	if parsed["has_active_levels"] != true {
		t.Fatalf("has_active_levels got %v", parsed["has_active_levels"])
	}
	if parsed["price"] != float64(63000) {
		t.Fatalf("price got %v", parsed["price"])
	}
	if parsed["rsi14"] != float64(58) {
		t.Fatalf("rsi14 got %v", parsed["rsi14"])
	}

	levels, ok := parsed["levels"].([]any)
	if !ok || len(levels) != 1 {
		t.Fatalf("levels got %v", parsed["levels"])
	}
	lvl := levels[0].(map[string]any)
	if lvl["breakout_1h"] != float64(0.42) {
		t.Fatalf("breakout_1h got %v", lvl["breakout_1h"])
	}
	if lvl["breakout_24h"] != float64(0.71) {
		t.Fatalf("breakout_24h got %v", lvl["breakout_24h"])
	}
	// breakout_4h was absent — the field must be omitted.
	if _, present := lvl["breakout_4h"]; present {
		t.Fatalf("breakout_4h should be omitted, got %v", lvl["breakout_4h"])
	}
}

func TestBuildTickerPayloadNilMarket(t *testing.T) {
	row := &database.DetectorStateRow{
		Ticker:            "BTCUSDT",
		HasActiveLevels:   false,
		FilteredStateBlob: []byte(`{"levels":[]}`),
	}
	out, err := buildTickerPayload(row, nil)
	if err != nil {
		t.Fatalf("buildTickerPayload: %v", err)
	}
	// price and other market fields must be absent.
	if strings.Contains(out, `"price"`) {
		t.Fatalf("nil market should not produce price field: %s", out)
	}
	if !strings.Contains(out, `"ticker":"BTCUSDT"`) {
		t.Fatalf("ticker missing: %s", out)
	}
}

// ============================================================
//  langInstruction
// ============================================================

func TestLangInstruction(t *testing.T) {
	if !strings.Contains(langInstruction("ru"), "Russian") {
		t.Fatalf("ru instruction wrong")
	}
	if !strings.Contains(langInstruction("RU"), "Russian") {
		t.Fatalf("case-insensitive ru failed")
	}
	if !strings.Contains(langInstruction("en"), "English") {
		t.Fatalf("en instruction wrong")
	}
	if langInstruction("fr") != "" {
		t.Fatalf("unknown lang should be empty, got %q", langInstruction("fr"))
	}
}