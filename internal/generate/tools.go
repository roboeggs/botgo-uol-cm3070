package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"botgo/internal/database"
	"botgo/internal/market"

	"github.com/openai/openai-go"
)

const maxActiveTickers = 3

type ToolExecutor struct {
	cache *Cache
}

func NewToolExecutor(cache *Cache) *ToolExecutor {
	return &ToolExecutor{cache: cache}
}

func declGetActiveTickers() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name: "get_active_tickers",
		Description: openai.String(`
			Returns cryptocurrency tickers that currently have active
			support and resistance levels in the database.

			Use this when the user asks which cryptocurrencies currently
			have available active levels or asks for potentially interesting
			cryptocurrencies without specifying a ticker.
		`),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{},
			"additionalProperties": false,
		},
	}
}

func declGetTopMLTickers() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name: "get_top_ml_tickers",
		Description: openai.String(`
			Returns the top tickers ranked by the ML model's breakout
			probability. Use this when the user asks what is interesting,
			hot, or worth watching, without naming a specific ticker.
		`),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{},
			"additionalProperties": false,
		},
	}
}

func declGetTickerLevels() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name: "get_ticker_levels",
		Description: openai.String(`
			Returns active support and resistance levels for a specific
			cryptocurrency ticker from the database.
		`),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"ticker": map[string]any{
					"type":        "string",
					"description": "Trading pair, for example BTCUSDT",
				},
			},
			"required": []string{"ticker"},
			"additionalProperties": false,
		},
	}
}

func declGetMarketData() openai.FunctionDefinitionParam {
	return openai.FunctionDefinitionParam{
		Name: "get_market_data",
		Description: openai.String(
			"Returns current market data for a specific cryptocurrency ticker. " +
				"Includes: current price, 24h change (%), 24h high/low, 24h volume, " +
				"EMA(20) and RSI(14) computed on 4h candles.",
		),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"ticker": map[string]any{
					"type":        "string",
					"description": "Trading pair, for example BTCUSDT",
				},
			},
			"required": []string{"ticker"},
			"additionalProperties": false,
		},
	}
}

func getTickerArg(args map[string]any) (string, error) {
	value, ok := args["ticker"]
	if !ok {
		return "", fmt.Errorf("missing required argument: ticker")
	}

	ticker, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("ticker must be a string")
	}

	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return "", fmt.Errorf("ticker is empty")
	}

	return ticker, nil
}

func filterAvailableTickers(tickers []string) []string {
	if len(tickers) <= maxActiveTickers {
		return tickers
	}
	return tickers[:maxActiveTickers]
}

func activeTickersResponse(all []string, cached bool) map[string]any {
	slice := filterAvailableTickers(all)
	return map[string]any{
		"tickers": slice,
		"count": len(slice),
		"total_count": len(all),
		"truncated": len(all) > len(slice),
		"cached": cached,
	}
}

func (e *ToolExecutor) getActiveTickers(ctx context.Context) (map[string]any, error) {
	if tickers, ok := e.cache.GetActiveTickers(); ok {
		slog.Info("active tickers loaded from cache", "count", len(tickers))
		return activeTickersResponse(tickers, true), nil
	}

	slog.Info("loading active tickers from database")

	tickers, err := database.GetTickerActiveLevels(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active tickers from database: %w", err)
	}
	slog.Info("active tickers loaded from database", "count", len(tickers))

	e.cache.SetActiveTickers(tickers)
	return activeTickersResponse(tickers, false), nil
}

func (e *ToolExecutor) getTopMLTickers(ctx context.Context) (map[string]any, error) {
	stats, err := database.GetTopByBreakout(ctx, 5)
	if err != nil {
		return nil, fmt.Errorf("get top by breakout: %w", err)
	}
	if len(stats) == 0 {
		return map[string]any{
			"tickers": []string{},
			"count":   0,
		}, nil
	}

	limit := maxActiveTickers
	if len(stats) < limit {
		limit = len(stats)
	}
	tickers := make([]string, 0, limit)
	for _, st := range stats[:limit] {
		tickers = append(tickers, st.Ticker)
	}
	return map[string]any{
		"tickers": tickers,
		"count": len(tickers),
	}, nil
}

func (e *ToolExecutor) getTickerLevels(ctx context.Context, args map[string]any) (map[string]any, error) {
	ticker, err := getTickerArg(args)
	if err != nil {
		return nil, err
	}

	if row, ok := e.cache.GetLevels(ticker); ok {
		slog.Info("ticker levels loaded from cache", "ticker", ticker)
		return formatTickerLevels(row, true)
	}

	slog.Info("loading ticker levels from database", "ticker", ticker)

	row, err := database.GetTicker(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("get ticker levels from database: %w", err)
	}
	slog.Info("ticker levels loaded from database", "ticker", ticker, "found", row != nil)

	if row == nil {
		return map[string]any{
			"ticker": ticker,
			"found": false,
		}, nil
	}

	e.cache.SetLevels(ticker, row)
	return formatTickerLevels(row, false)
}

func formatTickerLevels(row *database.DetectorStateRow, cached bool) (map[string]any, error) {
	var state any

	if len(row.FilteredStateBlob) > 0 {
		if err := json.Unmarshal(row.FilteredStateBlob, &state); err != nil {
			return nil, fmt.Errorf("decode state blob: %w", err)
		}
	}

	return map[string]any{
		"ticker": row.Ticker,
		"saved_at": row.SavedAt,
		"has_active_levels": row.HasActiveLevels,
		"state": state,
		"cached": cached,
	}, nil
}

// formatMarketData — a single response for get_market_data regardless of the source.
func formatMarketData(data market.MarketData, cached bool) map[string]any {
	return map[string]any{
		"ticker": data.Ticker,
		"price": data.Price,
		"ema20": data.EMA20,
		"rsi14": data.RSI14,
		"high24h": data.High24h,
		"low24h": data.Low24h,
		"change24h": data.Change24h,
		"volume24h": data.Volume24h,
		"source": data.Source,
		"updated_at": data.UpdatedAt,
		"cached": cached,
	}
}

func (e *ToolExecutor) getMarketData(ctx context.Context, args map[string]any) (map[string]any, error) {
	ticker, err := getTickerArg(args)
	if err != nil {
		return nil, err
	}

	if data, ok := e.cache.GetMarket(ticker); ok {
		slog.Info("market data loaded from cache", "ticker", ticker)
		return formatMarketData(data, true), nil
	}

	slog.Info("loading market data from exchange", "ticker", ticker)

	data, err := market.GetMarketData(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("get market data: %w", err)
	}

	slog.Info("market data loaded from exchange", "ticker", ticker)
	e.cache.SetMarket(ticker, data)

	return formatMarketData(data, false), nil
}

func (e *ToolExecutor) Execute(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	slog.Info("tool call", "tool", name, "args", args)

	allowed := false
	for _, t := range registry {
		if t.Name == name && t.Enabled {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("tool %s is not available", name)
	}

	switch name {
	case "get_active_tickers":
		return e.getActiveTickers(ctx)
	case "get_top_ml_tickers":
		return e.getTopMLTickers(ctx)
	case "get_ticker_levels":
		return e.getTickerLevels(ctx, args)
	case "get_market_data":
		return e.getMarketData(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}