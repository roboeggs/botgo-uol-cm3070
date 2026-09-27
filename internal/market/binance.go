package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	binanceBaseURL = "https://api.binance.com"
	KlineInterval = "4h"
	KlineLimit = 100 // with a margin for EMA(20)/RSI(14)
	binanceTimeout = 10 * time.Second
)

type binanceClient struct {
	http *http.Client
	baseURL string
}

func newBinanceClient() *binanceClient {
	return &binanceClient{
		http: &http.Client{Timeout: binanceTimeout},
		baseURL: binanceBaseURL,
	}
}

// --- Klines ---

func (c *binanceClient) getKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {

	url := fmt.Sprintf(
		"%s/api/v3/klines?symbol=%s&interval=%s&limit=%d",
		c.baseURL, symbol, interval, limit,
	)

	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	// Binance response — an array of arrays, where numbers come as strings.
	var raw [][]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode klines: %w", err)
	}

	klines := make([]Kline, 0, len(raw))
	for i, row := range raw {
		k, err := parseKlineRow(row)
		if err != nil {
			return nil, fmt.Errorf("parse kline row %d: %w", i, err)
		}
		klines = append(klines, k)
	}

	return klines, nil
}

func parseKlineRow(row []any) (Kline, error) {
	if len(row) < 7 {
		return Kline{}, fmt.Errorf("unexpected row length: %d", len(row))
	}

	openTime, err := toInt64(row[0])
	if err != nil {
		return Kline{}, fmt.Errorf("openTime: %w", err)
	}
	open, err := toFloat(row[1])
	if err != nil {
		return Kline{}, fmt.Errorf("open: %w", err)
	}
	high, err := toFloat(row[2])
	if err != nil {
		return Kline{}, fmt.Errorf("high: %w", err)
	}
	low, err := toFloat(row[3])
	if err != nil {
		return Kline{}, fmt.Errorf("low: %w", err)
	}
	close_, err := toFloat(row[4])
	if err != nil {
		return Kline{}, fmt.Errorf("close: %w", err)
	}
	volume, err := toFloat(row[5])
	if err != nil {
		return Kline{}, fmt.Errorf("volume: %w", err)
	}
	closeTime, err := toInt64(row[6])
	if err != nil {
		return Kline{}, fmt.Errorf("closeTime: %w", err)
	}

	return Kline{
		OpenTime: openTime,
		Open: open,
		High: high,
		Low: low,
		Close: close_,
		Volume: volume,
		CloseTime: closeTime,
	}, nil
}

// 24h ticker

type ticker24hResponse struct {
	Symbol string `json:"symbol"`
	LastPrice string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	HighPrice string `json:"highPrice"`
	LowPrice string `json:"lowPrice"`
	Volume string `json:"volume"`
	QuoteVolume string `json:"quoteVolume"`
}

func (c *binanceClient) get24hTicker(ctx context.Context,symbol string) (*ticker24hResponse, error) {

	url := fmt.Sprintf(
		"%s/api/v3/ticker/24hr?symbol=%s",
		c.baseURL, symbol,
	)

	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var t ticker24hResponse
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("decode 24h ticker: %w", err)
	}

	return &t, nil
}

// General GET with Binance error parsing

type binanceErrorBody struct {
	Code int `json:"code"`
	Msg string `json:"msg"`
}

func (c *binanceClient) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var be binanceErrorBody
		if jsonErr := json.Unmarshal(body, &be); jsonErr == nil && be.Msg != "" {
			return nil, fmt.Errorf(
				"binance: status=%d code=%d msg=%q",
				resp.StatusCode, be.Code, be.Msg,
			)
		}
		return nil, fmt.Errorf(
			"binance: status=%d body=%s",
			resp.StatusCode, string(body),
		)
	}

	return body, nil
}

// Numerical helpers

func toFloat(v any) (float64, error) {
	switch x := v.(type) {
	case string:
		return strconv.ParseFloat(x, 64)
	case float64:
		return x, nil
	}
	return 0, fmt.Errorf("unexpected number type %T", v)
}

func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case float64:
		return int64(x), nil
	case string:
		return strconv.ParseInt(x, 10, 64)
	}
	return 0, fmt.Errorf("unexpected int type %T", v)
}