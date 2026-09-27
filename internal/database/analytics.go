package database

import (
	"context"
	"time"
)

// TickerMLStat — summary statistics for the ticker, extracted from state_blob.
type TickerMLStat struct {
	Ticker string
	LevelsCount int
	MaxTouches int
	MaxBreakout1h float64
	MaxBreakout4h float64
	MaxBreakout24h float64
	LatestTouchTime *time.Time
}

// statsCTE — a general CTE that parses the state_blob of each ticker
// and calculates the maximum values by level. Used in all functions below.
const statsCTE = `
WITH stats AS (
	SELECT
		ds.ticker,
		COALESCE(jsonb_array_length(ds.state_blob->'confirmed_support_levels'), 0)
			+ COALESCE(jsonb_array_length(ds.state_blob->'confirmed_resistance_levels'), 0)
			AS levels_count,
		GREATEST(
			COALESCE((
				SELECT MAX((lvl->>'touch_count')::int)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_support_levels', '[]'::jsonb)
				) lvl
			), 0),
			COALESCE((
				SELECT MAX((lvl->>'touch_count')::int)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_resistance_levels', '[]'::jsonb)
				) lvl
			), 0)
		) AS max_touches,
		GREATEST(
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_1h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_support_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_1h' IS NOT NULL
			), 0),
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_1h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_resistance_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_1h' IS NOT NULL
			), 0)
		) AS max_breakout_1h,
		GREATEST(
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_4h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_support_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_4h' IS NOT NULL
			), 0),
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_4h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_resistance_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_4h' IS NOT NULL
			), 0)
		) AS max_breakout_4h,
		GREATEST(
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_24h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_support_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_24h' IS NOT NULL
			), 0),
			COALESCE((
				SELECT MAX((lvl->'ml_prediction'->'breakout_24h'->>'probability')::float8)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_resistance_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->'ml_prediction'->'breakout_24h' IS NOT NULL
			), 0)
		) AS max_breakout_24h,
		GREATEST(
			COALESCE((
				SELECT MAX((lvl->>'last_touch_time')::timestamptz)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_support_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->>'last_touch_time' IS NOT NULL
			), '1970-01-01'::timestamptz),
			COALESCE((
				SELECT MAX((lvl->>'last_touch_time')::timestamptz)
				FROM jsonb_array_elements(
					COALESCE(ds.state_blob->'confirmed_resistance_levels', '[]'::jsonb)
				) lvl
				WHERE lvl->>'last_touch_time' IS NOT NULL
			), '1970-01-01'::timestamptz)
		) AS latest_touch_time
	FROM public.detector_state ds
	WHERE ds.has_active_levels = TRUE
)
`

const statsSelect = `
SELECT
	ticker, levels_count, max_touches,
	max_breakout_1h, max_breakout_4h, max_breakout_24h,
	latest_touch_time
FROM stats
WHERE levels_count > 0
`

// GetTopByBreakout — top N by maximum probability of a breakout within 4 hours.
func GetTopByBreakout(ctx context.Context, limit int) ([]TickerMLStat, error) {
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	query := statsCTE + statsSelect + ` ORDER BY max_breakout_4h DESC LIMIT $1;`
	return scanStats(ctx, query, limit)
}

// GetTickersByTouches — top-N by the number of touches at the strongest level.
func GetTickersByTouches(ctx context.Context, minTouches, limit int) ([]TickerMLStat, error) {
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	if minTouches < 0 {
		minTouches = 0
	}
	query := statsCTE + statsSelect + ` AND max_touches >= $1 ORDER BY max_touches DESC LIMIT $2;`
	return scanStats(ctx, query, minTouches, limit)
}

// GetTickersByFreshness — top-N by the time of the last touch.
func GetTickersByFreshness(ctx context.Context, hours, limit int) ([]TickerMLStat, error) {
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	if hours < 1 {
		hours = 48
	}
	query := statsCTE + statsSelect + `
		AND latest_touch_time >= now() - make_interval(hours => $1)
		ORDER BY latest_touch_time DESC LIMIT $2;`
	return scanStats(ctx, query, hours, limit)
}

func scanStats(ctx context.Context, query string, args ...interface{}) ([]TickerMLStat, error) {
	rows, err := DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TickerMLStat
	for rows.Next() {
		var s TickerMLStat
		if err := rows.Scan(
			&s.Ticker,
			&s.LevelsCount,
			&s.MaxTouches,
			&s.MaxBreakout1h,
			&s.MaxBreakout4h,
			&s.MaxBreakout24h,
			&s.LatestTouchTime,
		); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}