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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/config"
	"github.com/evgslyusar/shortlink/internal/telegram"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck
	logger = logger.With(zap.String("service", "slinkbot"))

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bot := telegram.NewHandler(logger, cfg.TelegramBotToken)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /telegram/webhook", bot.HandleWebhook)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
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
		logger.Info("slinkbot starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("slinkbot error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down slinkbot")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("slinkbot shutdown error", zap.Error(err))
	}
	logger.Info("slinkbot stopped")
}
