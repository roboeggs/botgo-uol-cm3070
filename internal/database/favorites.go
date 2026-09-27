package database

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrFavoriteLimitReached = errors.New("favorites limit reached")
	ErrInvalidTicker = errors.New("invalid ticker format")
)

// tickerRe — the same format as in the CHECK constraint, to avoid sending
// invalid data to the database.
var tickerRe = regexp.MustCompile(`^[A-Z0-9]{1,12}USDT$`)

const maxFavoritesPerUser = 100

// NormalizeTicker converts the user’s input to a canonical form.
// "btc" → "BTCUSDT", "BTCUSDT" → "BTCUSDT", " btcusdt " → "BTCUSDT".
func NormalizeTicker(raw string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(raw))
	t = strings.TrimPrefix(t, "/")
	if t == "" {
		return "", ErrInvalidTicker
	}
	if !strings.HasSuffix(t, "USDT") {
		t += "USDT"
	}
	if !tickerRe.MatchString(t) {
		return "", ErrInvalidTicker
	}
	return t, nil
}

// AddFavorite adds the ticker to the user’s favorites.
// Idempotent: a repeated call is not an error.
func AddFavorite(ctx context.Context, userID int64, rawTicker string) error {
	ticker, err := NormalizeTicker(rawTicker)
	if err != nil {
		return err
	}

	// Check the limit only if the record doesn’t exist yet.
	var exists bool
	err = DB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM user_favorites WHERE user_id = $1 AND ticker = $2)`,
		userID, ticker,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check favorite exists: %w", err)
	}
	if exists {
		return nil // already in favorites
	}

	var count int
	err = DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_favorites WHERE user_id = $1`, userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("count favorites: %w", err)
	}
	if count >= maxFavoritesPerUser {
		return ErrFavoriteLimitReached
	}

	_, err = DB.ExecContext(ctx,
		`INSERT INTO user_favorites (user_id, ticker) VALUES ($1, $2)
		 ON CONFLICT (user_id, ticker) DO NOTHING`,
		userID, ticker,
	)
	if err != nil {
		return fmt.Errorf("insert favorite: %w", err)
	}
	return nil
}

// RemoveFavorite removes a ticker from the favorites.
func RemoveFavorite(ctx context.Context, userID int64, rawTicker string) error {
	ticker, err := NormalizeTicker(rawTicker)
	if err != nil {
		return err
	}
	_, err = DB.ExecContext(ctx,
		`DELETE FROM user_favorites WHERE user_id = $1 AND ticker = $2`,
		userID, ticker,
	)
	return err
}

// GetFavorites returns a list of the user’s favorite tickers, with the most recent ones at the top.
func GetFavorites(ctx context.Context, userID int64) ([]string, error) {
	rows, err := DB.QueryContext(ctx,
		`SELECT ticker FROM user_favorites WHERE user_id = $1 ORDER BY added_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("query favorites: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// IsFavorite checks whether a ticker is in the favorites.
func IsFavorite(ctx context.Context, userID int64, rawTicker string) (bool, error) {
	ticker, err := NormalizeTicker(rawTicker)
	if err != nil {
		return false, err
	}
	var exists bool
	err = DB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM user_favorites WHERE user_id = $1 AND ticker = $2)`,
		userID, ticker,
	).Scan(&exists)
	return exists, err
}

// GetFavoritesShort — for the URL parameter fav=BTC,ETH,SUI (without USDT).
func GetFavoritesShort(ctx context.Context, userID int64) (string, error) {
	favs, err := GetFavorites(ctx, userID)
	if err != nil {
		return "", err
	}
	short := make([]string, 0, len(favs))
	for _, f := range favs {
		short = append(short, strings.TrimSuffix(f, "USDT"))
	}
	return strings.Join(short, ","), nil
}