package database

import (
	"context"
	"database/sql"
	"time"

	tele "gopkg.in/telebot.v4"
)

// User represents a record in the users table
type User struct {
	ID int64
	Username *string
	FirstName string
	LastName *string
	LanguageCode string
	IsBlocked bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpsertUser creates or updates a user in the database
// Returns the user's language (from the database) and an error
func UpsertUser(c tele.Context) (string, error) {
	sender := c.Sender()
	id := sender.ID
	username := nullString(sender.Username)
	firstName := sender.FirstName
	lastName := nullString(sender.LastName)

	query := `
		INSERT INTO users (id, username, first_name, last_name, language_code, updated_at)
		VALUES ($1, $2, $3, $4, COALESCE((SELECT language_code FROM users WHERE id = $1), 'en'), now())
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			updated_at = now()
		RETURNING language_code
	`

	var lang string
	err := DB.QueryRowContext(context.Background(), query,
		id, username, firstName, lastName,
	).Scan(&lang)
	if err != nil {
		return "en", err
	}
	return lang, nil
}

// GetLanguage returns the saved language of the user (without updating the data)
// Used when you only need to read the language
func GetLanguage(userID int64) (string, error) {
	var lang string
	query := `SELECT language_code FROM users WHERE id = $1`
	err := DB.QueryRowContext(context.Background(), query, userID).Scan(&lang)
	if err == sql.ErrNoRows {
		return "en", nil
	}
	if err != nil {
		return "en", err
	}
	return lang, nil
}

// UpdateLanguage updates the user's language in the database
func UpdateLanguage(userID int64, lang string) error {
	query := `UPDATE users SET language_code = $1, updated_at = now() WHERE id = $2`
	_, err := DB.ExecContext(context.Background(), query, lang, userID)
	return err
}

// nullString returns a pointer to a string or nil if the string is empty
// Used for fields that may be NULL in the database
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}