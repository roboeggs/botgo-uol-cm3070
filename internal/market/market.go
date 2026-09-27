package market

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MarketData — aggregated data for a single ticker,
// which is returned by LLM via tool get_market_data.
type MarketData struct {
	Ticker string
	Price float64
	EMA20 float64
	RSI14 float64
	High24h float64
	Low24h float64
	Change24h float64 // in percentage
	Volume24h float64 // base volume over 24h (in coin)
	Source string
	UpdatedAt time.Time
}

// Kline — one Binance candle.
type Kline struct {
	OpenTime int64
	Open float64
	High float64
	Low float64
	Close float64
	Volume float64
	CloseTime int64
}

func GetMarketData(ctx context.Context, ticker string) (MarketData, error) {
	ticker = strings.ToUpper(strings.TrimSpace(ticker))
	if ticker == "" {
		return MarketData{}, fmt.Errorf("empty ticker")
	}

	client := newBinanceClient()

	var (
		wg sync.WaitGroup
		klines []Kline
		ticker24 *ticker24hResponse
		klineErr error
		tickerErr error
	)

	wg.Add(2)
	go func() {
		defer wg.Done()
		klines, klineErr = client.getKlines(ctx, ticker, KlineInterval, KlineLimit)
	}()
	go func() {
		defer wg.Done()
		ticker24, tickerErr = client.get24hTicker(ctx, ticker)
	}()
	wg.Wait()

	if klineErr != nil {
		return MarketData{}, fmt.Errorf("fetch klines for %s: %w", ticker, klineErr)
	}
	if tickerErr != nil {
		return MarketData{}, fmt.Errorf("fetch 24h ticker for %s: %w", ticker, tickerErr)
	}
	if len(klines) == 0 {
		return MarketData{}, fmt.Errorf("no klines returned for %s", ticker)
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	price, err := strconv.ParseFloat(ticker24.LastPrice, 64)
	if err != nil {
		return MarketData{}, fmt.Errorf("parse lastPrice: %w", err)
	}
	change24, err := strconv.ParseFloat(ticker24.PriceChangePercent, 64)
	if err != nil {
		return MarketData{}, fmt.Errorf("parse priceChangePercent: %w", err)
	}
	high24, err := strconv.ParseFloat(ticker24.HighPrice, 64)
	if err != nil {
		return MarketData{}, fmt.Errorf("parse highPrice: %w", err)
	}
	low24, err := strconv.ParseFloat(ticker24.LowPrice, 64)
	if err != nil {
		return MarketData{}, fmt.Errorf("parse lowPrice: %w", err)
	}
	volume24, err := strconv.ParseFloat(ticker24.Volume, 64)
	if err != nil {
		return MarketData{}, fmt.Errorf("parse volume: %w", err)
	}

	return MarketData{
		Ticker: ticker,
		Price: price,
		EMA20: CalculateEMA(closes, 20),
		RSI14: CalculateRSI(closes, 14),
		High24h: high24,
		Low24h: low24,
		Change24h: change24,
		Volume24h: volume24,
		Source: "binance",
		UpdatedAt: time.Now().UTC(),
	}, nil
}