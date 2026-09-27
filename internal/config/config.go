package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	BotToken string
	ApiKey string
	AdminID  int64
	DatabaseURL string
	WebAppURL string
}

func errorMsg(name string) string { return name + " env variable is not set" }

func Load() (*Config, error) {
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, errors.New(errorMsg("BOT_TOKEN"))
	}

	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		return nil, errors.New(errorMsg("OPENAI_API_KEY"))
	}

	adminIDStr := os.Getenv("ADMIN_ID")
	if adminIDStr == "" {
		return nil, errors.New(errorMsg("ADMIN_ID"))
	}

	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		return nil, errors.New("ADMIN_ID must be a valid integer")
	}

	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		return nil, errors.New(errorMsg("DATABASE_URL"))
	}

	webAppURL := os.Getenv("WEBAPP_URL")

	if webAppURL == "" {
		return nil, errors.New(errorMsg("WEBAPP_URL"))
	}


	return &Config{
		BotToken: token,
		ApiKey: apiKey,
		AdminID:  adminID,
		DatabaseURL: dbURL,
		WebAppURL: webAppURL,
	}, nil
}