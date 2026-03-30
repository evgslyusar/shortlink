// Slink Bot — Telegram interface for the URL shortener.
// Telegram: @SlinkBot
//
// Commands:
//   (plain URL) → shorten and return short link
//   /stats <slug> → click count for a link
//   /list → 5 most recent links
//   /account → link Telegram account to registered profile

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/evgslyusar/shortlink/internal/config"
	"github.com/evgslyusar/shortlink/internal/telegram"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With(slog.String("service", "slinkbot"))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if cfg.TelegramToken == "" {
		logger.Error("TELEGRAM_BOT_TOKEN is required")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bot := telegram.NewHandler(logger, cfg.TelegramToken)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /telegram/webhook", bot.HandleWebhook)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf("%s:%d", cfg.BotHost, cfg.BotPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("slinkbot starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("slinkbot error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down slinkbot")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("slinkbot shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("slinkbot stopped")
}
