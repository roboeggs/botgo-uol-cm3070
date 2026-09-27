package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type DetectorStateRow struct {
	Ticker string `json:"ticker"`
	SavedAt *time.Time `json:"saved_at"`
	LastProcessedTime *int64 `json:"last_processed_time"`
	HasActiveLevels bool `json:"has_active_levels"`
	FilteredStateBlob json.RawMessage `json:"state_blob"`
}

func GetTicker(ctx context.Context, ticker string) (*DetectorStateRow, error) {
	query := `
		SELECT
			ds.ticker,
			ds.saved_at,
			pl.last_processed_time,
			ds.has_active_levels,
			jsonb_build_object(
				'last_processed_time', ds.state_blob->'last_processed_time',
				'_level_id_seq',       ds.state_blob->'_level_id_seq',
				'levels', COALESCE((
					SELECT jsonb_agg(
						(lvl - 'samples') || jsonb_build_object(
							'touch_times',
							COALESCE(
								(SELECT s->'touch_times'
								 FROM jsonb_array_elements(COALESCE(lvl->'samples','[]'::jsonb)) s
								 ORDER BY s->>'prediction_time' DESC NULLS LAST
								 LIMIT 1),
								'[]'::jsonb
							)
						)
					)
					FROM (
						SELECT DISTINCT ON (lvl->>'level_id') lvl
						FROM (
							SELECT jsonb_array_elements(
								COALESCE(ds.state_blob->'confirmed_support_levels','[]'::jsonb)
							) AS lvl
							UNION ALL
							SELECT jsonb_array_elements(
								COALESCE(ds.state_blob->'confirmed_resistance_levels','[]'::jsonb)
							) AS lvl
						) all_lvls
						ORDER BY
							lvl->>'level_id',
							COALESCE(lvl->>'last_touch_time','') DESC
					) deduped
				), '[]'::jsonb)
			) AS filtered_state_blob
		FROM public.detector_state ds
		LEFT JOIN public.processing_log pl ON pl.ticker = ds.ticker
		WHERE ds.ticker = $1;
	`

	var row DetectorStateRow
	err := DB.QueryRowContext(ctx, query, ticker).Scan(
		&row.Ticker,
		&row.SavedAt,
		&row.LastProcessedTime,
		&row.HasActiveLevels,
		&row.FilteredStateBlob,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func GetTickerActiveLevels(ctx context.Context) ([]string, error) {
	query := `
		SELECT ticker
		FROM public.detector_state
		WHERE has_active_levels = TRUE;
	`

	rows, err := DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			return nil, err
		}
		tickers = append(tickers, ticker)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return tickers, nil
}