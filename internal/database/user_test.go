package database

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
)

// mockContext implements tele.Context for tests.
type mockContext struct {
	tele.Context
	sender *tele.User
}

func (m *mockContext) Sender() *tele.User {
	return m.sender
}

// Test for UpsertUser (insert new user).
func TestUpsertUser_Insert(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	rows := sqlmock.NewRows([]string{"language_code"}).AddRow("ru")
	// The query uses 4 arguments: id, username, first_name, last_name.
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(int64(123), sqlmock.AnyArg(), "Test", sqlmock.AnyArg()).
		WillReturnRows(rows)

	user := &tele.User{
		ID:        123,
		Username:  "",
		FirstName: "Test",
		LastName:  "",
	}
	ctx := &mockContext{sender: user}

	lang, err := UpsertUser(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "ru", lang)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test for UpsertUser (update existing user).
func TestUpsertUser_Update(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	rows := sqlmock.NewRows([]string{"language_code"}).AddRow("en")
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(int64(456), "user", "John", "Doe").
		WillReturnRows(rows)

	user := &tele.User{
		ID:        456,
		Username:  "user",
		FirstName: "John",
		LastName:  "Doe",
	}
	ctx := &mockContext{sender: user}

	lang, err := UpsertUser(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "en", lang)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test for GetLanguage (existing user).
func TestGetLanguage_Exists(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	rows := sqlmock.NewRows([]string{"language_code"}).AddRow("ru")
	mock.ExpectQuery(`SELECT language_code FROM users WHERE id = \$1`).
		WithArgs(int64(789)).
		WillReturnRows(rows)

	lang, err := GetLanguage(789)
	assert.NoError(t, err)
	assert.Equal(t, "ru", lang)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test for GetLanguage (user not found).
func TestGetLanguage_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	mock.ExpectQuery(`SELECT language_code FROM users WHERE id = \$1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	lang, err := GetLanguage(999)
	assert.NoError(t, err)
	assert.Equal(t, "en", lang)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Test for UpdateLanguage.
func TestUpdateLanguage(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	oldDB := DB
	DB = db
	defer func() { DB = oldDB }()

	mock.ExpectExec(`UPDATE users SET language_code = \$1, updated_at = now\(\) WHERE id = \$2`).
		WithArgs("en", int64(123)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateLanguage(123, "en")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}