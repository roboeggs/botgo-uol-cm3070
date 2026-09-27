package database

import (
	"database/sql"
	"log/slog"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var DB *sql.DB

func InitDB(dsn string) error {
	var err error
	DB, err = sql.Open("pgx", dsn)
	
	if err != nil {
		slog.Error("Failed to open database connection", "error", err)
		return err
	}

	DB.SetMaxOpenConns(2)
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(time.Hour)


	if err := DB.Ping(); err != nil {
		return err
	}

	slog.Info("Database connectes successfully")
	return nil
}

func CloseDB() error {
	if DB != nil {
		err := DB.Close()
		if err == nil {
			slog.Info("Database closed")
		}
		return err
	}
	return nil
}