package main

import (
	"log/slog"
	"time"
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"

	"botgo/internal/config"
	"botgo/internal/handlers"
	"botgo/internal/database"
	"botgo/internal/generate"
	"botgo/internal/i18n"
)

func main() {
	opts := &slog.HandlerOptions{ Level: slog.LevelDebug }

	if level := os.Getenv("LOG_LEVEL"); level == "debug" {
		opts.Level = slog.LevelDebug
	}

	logger := slog.NewTextHandler(os.Stdout, opts)
	slog.SetDefault(slog.New(logger))

	slog.Info("Starting bot...")
	if err := godotenv.Load(); err !=nil {
		slog.Warn("No .env file found, continuing...", "error", err)
	}


	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("Config loaded", "admin_id")
	slog.Info("WebApp URL loaded", "url", cfg.WebAppURL)

	if err := database.InitDB(cfg.DatabaseURL); err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.CloseDB()

	aiService, err := generate.New(cfg.ApiKey)
	if err != nil {
		slog.Error("Failed to create AI client", "error", err)
        os.Exit(1)
	}

	i18n.Init()

	pref := tele.Settings{
		Token: cfg.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
	}
	
	b, err := tele.NewBot(pref)
	if err !=nil {

		slog.Error("Bot initialization failed", "error", err)
		os.Exit(1)
	}


	handlers.Register(b, cfg.AdminID, aiService, cfg.WebAppURL)

	port := os.Getenv("PORT")
	if port == "" {
		port = "10000"
	}

	mux := http.NewServeMux()
	
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	staticDir := http.Dir("./static")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(staticDir)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Bot is running"))
	})

	srv := &http.Server{
		Addr: "0.0.0.0:" + port,
		Handler: mux,
	}

	go func() {
		slog.Info("Health server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health server error", "error", err)
		}
	}()

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	
	go func(){
		slog.Info("The bot has been successfully launched...")
		b.Start()
	}()


	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	slog.Info("Shutting down...")

    b.Stop()
    slog.Info("Bot stopped")
}