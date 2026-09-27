package market

import (
	"math"
	"testing"
)

func TestCalculateEMA(t *testing.T) {
	// Flat series: EMA should match the price.
	flat := make([]float64, 30)
	for i := range flat {
		flat[i] = 100
	}
	got := CalculateEMA(flat, 20)
	if math.Abs(got-100) > 1e-9 {
		t.Errorf("flat EMA = %v, want 100", got)
	}

	// Insufficient data → 0.
	if CalculateEMA([]float64{1, 2, 3}, 20) != 0 {
		t.Errorf("expected 0 for insufficient data")
	}
}

func TestCalculateRSI(t *testing.T) {
	// Monotonic uptrend → RSI close to 100.
	up := make([]float64, 30)
	for i := range up {
		up[i] = 100 + float64(i)
	}
	rsiUp := CalculateRSI(up, 14)
	if rsiUp < 99 {
		t.Errorf("uptrend RSI = %v, want ~100", rsiUp)
	}

	// Monotonic downtrend → RSI close to 0.
	down := make([]float64, 30)
	for i := range down {
		down[i] = 100 - float64(i)
	}
	rsiDown := CalculateRSI(down, 14)
	if rsiDown > 1 {
		t.Errorf("downtrend RSI = %v, want ~0", rsiDown)
	}

	// Insufficient data → 0.
	if CalculateRSI([]float64{1, 2, 3}, 14) != 0 {
		t.Errorf("expected 0 for insufficient data")
	}
}