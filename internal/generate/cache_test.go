package generate

import (
	"testing"
	"time"

	"botgo/internal/database"
	"botgo/internal/market"
)

func TestCacheMarket(t *testing.T) {
	cache := NewCache()

	data := market.MarketData{
		Ticker: "BTCUSDT",
		Price: 100000,
		EMA20: 99000,
		RSI14: 55,
	}

	cache.SetMarket("BTCUSDT", data)

	got, ok := cache.GetMarket("BTCUSDT")
	if !ok {
		t.Fatal("expected market data in cache")
	}

	if got.Ticker != data.Ticker {
		t.Errorf("ticker = %q, want %q", got.Ticker, data.Ticker)
	}

	if got.Price != data.Price {
		t.Errorf("price = %v, want %v", got.Price, data.Price)
	}
}

func TestCacheMarketMiss(t *testing.T) {
	cache := NewCache()

	_, ok := cache.GetMarket("BTCUSDT")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestCacheMarketDifferentTickers(t *testing.T) {
	cache := NewCache()

	btc := market.MarketData{
		Ticker: "BTCUSDT",
		Price: 100000,
	}

	eth := market.MarketData{
		Ticker: "ETHUSDT",
		Price: 4000,
	}

	cache.SetMarket("BTCUSDT", btc)
	cache.SetMarket("ETHUSDT", eth)

	gotBTC, ok := cache.GetMarket("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT should be cached")
	}

	gotETH, ok := cache.GetMarket("ETHUSDT")
	if !ok {
		t.Fatal("ETHUSDT should be cached")
	}

	if gotBTC.Price != 100000 {
		t.Errorf("BTC price = %v, want 100000", gotBTC.Price)
	}

	if gotETH.Price != 4000 {
		t.Errorf("ETH price = %v, want 4000", gotETH.Price)
	}
}

func TestCacheActiveTickers(t *testing.T) {
	cache := NewCache()

	tickers := []string{
		"BTCUSDT",
		"ETHUSDT",
		"SOLUSDT",
	}

	cache.SetActiveTickers(tickers)

	got, ok := cache.GetActiveTickers()
	if !ok {
		t.Fatal("expected active tickers in cache")
	}

	if len(got) != len(tickers) {
		t.Fatalf("got %d tickers, want %d", len(got), len(tickers))
	}

	for i := range tickers {
		if got[i] != tickers[i] {
			t.Errorf("ticker[%d] = %q, want %q", i, got[i], tickers[i])
		}
	}
}

func TestCacheActiveTickersCopiesSlice(t *testing.T) {
	cache := NewCache()

	tickers := []string{
		"BTCUSDT",
		"ETHUSDT",
	}

	cache.SetActiveTickers(tickers)

	// Mutate the original slice.
	tickers[0] = "CHANGED"

	got, ok := cache.GetActiveTickers()
	if !ok {
		t.Fatal("expected active tickers in cache")
	}

	if got[0] != "BTCUSDT" {
		t.Errorf(
			"cache was modified through original slice: got %q",
			got[0],
		)
	}
}

func TestCacheLevels(t *testing.T) {
	cache := NewCache()

	savedAt := time.Now()

	row := &database.DetectorStateRow{
		Ticker: "BTCUSDT",
		SavedAt: &savedAt,
		HasActiveLevels: true,
	}

	cache.SetLevels("BTCUSDT", row)

	got, ok := cache.GetLevels("BTCUSDT")
	if !ok {
		t.Fatal("expected levels in cache")
	}

	if got != row {
		t.Fatal("expected cached pointer to be returned")
	}

	if got.Ticker != "BTCUSDT" {
		t.Errorf("ticker = %q, want BTCUSDT", got.Ticker)
	}

	if !got.HasActiveLevels {
		t.Error("expected HasActiveLevels=true")
	}
}

func TestCacheLevelsDifferentTickers(t *testing.T) {
	cache := NewCache()

	btc := &database.DetectorStateRow{
		Ticker:"BTCUSDT",
	}

	eth := &database.DetectorStateRow{
		Ticker:"ETHUSDT",
	}

	cache.SetLevels("BTCUSDT", btc)
	cache.SetLevels("ETHUSDT", eth)

	gotBTC, ok := cache.GetLevels("BTCUSDT")
	if !ok {
		t.Fatal("BTCUSDT should be cached")
	}

	gotETH, ok := cache.GetLevels("ETHUSDT")
	if !ok {
		t.Fatal("ETHUSDT should be cached")
	}

	if gotBTC.Ticker != "BTCUSDT" {
		t.Errorf("BTC ticker = %q", gotBTC.Ticker)
	}

	if gotETH.Ticker != "ETHUSDT" {
		t.Errorf("ETH ticker = %q", gotETH.Ticker)
	}
}