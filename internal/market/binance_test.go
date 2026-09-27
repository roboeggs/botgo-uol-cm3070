package market

import (
	"context"
	"testing"
)

func TestGetMarketDataLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Binance test in short mode")
	}

	data, err := GetMarketData(context.Background(), "btcusdt")
	if err != nil {
		t.Fatalf("GetMarketData: %v", err)
	}

	if data.Ticker != "BTCUSDT" {
		t.Errorf("ticker = %q, want BTCUSDT", data.Ticker)
	}
	if data.Price <= 0 {
		t.Errorf("price = %v, want > 0", data.Price)
	}
	if data.EMA20 <= 0 {
		t.Errorf("ema20 = %v, want > 0", data.EMA20)
	}
	if data.RSI14 < 0 || data.RSI14 > 100 {
		t.Errorf("rsi14 = %v, want within [0,100]", data.RSI14)
	}

	t.Logf("BTCUSDT price=%v ema20=%v rsi14=%v change24h=%v%%",
		data.Price, data.EMA20, data.RSI14, data.Change24h)
}