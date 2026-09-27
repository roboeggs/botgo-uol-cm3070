package market

// CalculateEMA calculates the exponential moving average.
// Returns 0 if there is less data than period.
func CalculateEMA(values []float64, period int) float64 {
	if period <= 0 || len(values) < period {
		return 0
	}

	k := 2.0 / float64(period+1)

	// Seed: SMA of the first `period` values.
	var seed float64
	for i := 0; i < period; i++ {
		seed += values[i]
	}
	seed /= float64(period)

	ema := seed
	for i := period; i < len(values); i++ {
		ema = values[i]*k + ema*(1-k)
	}
	return ema
}

// CalculateRSI calculates RSI using Wilder’s method.
// Requires at least period+1 points (period of price changes).
// Returns 0 if there is insufficient data.
func CalculateRSI(values []float64, period int) float64 {
	if period <= 0 || len(values) < period+1 {
		return 0
	}

	// The first avgGain / avgLoss is the simple average over the first `period` changes.
	var gainSum, lossSum float64
	for i := 1; i <= period; i++ {
		delta := values[i] - values[i-1]
		if delta > 0 {
			gainSum += delta
		} else {
			lossSum -= delta
		}
	}

	avgGain := gainSum / float64(period)
	avgLoss := lossSum / float64(period)

	// Wilder smoothing for the remaining points.
	for i := period + 1; i < len(values); i++ {
		delta := values[i] - values[i-1]
		var gain, loss float64
		if delta > 0 {
			gain = delta
		} else {
			loss = -delta
		}

		avgGain = (avgGain*float64(period-1) + gain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
	}

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}