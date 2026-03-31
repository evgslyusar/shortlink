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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/config"
	"github.com/evgslyusar/shortlink/internal/repository"
	"github.com/evgslyusar/shortlink/internal/service"
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

	// Connect to PostgreSQL.
	dbPool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		logger.Fatal("failed to ping postgres", zap.Error(err))
	}
	logger.Info("connected to postgres")

	// Connect to Redis.
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Fatal("failed to parse redis url", zap.Error(err))
	}

	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("failed to ping redis", zap.Error(err))
	}
	logger.Info("connected to redis")

	// Set up dependencies.
	userRepo := repository.NewUserPostgres(dbPool)
	authSvc := service.NewAuthService(userRepo, userRepo, logger)

	linkRepo := repository.NewLinkPostgres(dbPool)
	linkCache := repository.NewLinkCache(rdb)
	linkSvc := service.NewLinkService(linkRepo, linkRepo, linkRepo, linkRepo, linkCache, logger)

	clickRepo := repository.NewClickPostgres(dbPool)
	clickSvc := service.NewClickService(clickRepo, clickRepo, linkRepo, logger)

	telegramRepo := repository.NewTelegramAccountPostgres(dbPool)

	tgSvc := telegram.NewService(
		telegramRepo, telegramRepo,
		authSvc,
		linkSvc, linkSvc, linkSvc,
		clickSvc,
		cfg.BaseURL, logger,
	)

	bot := telegram.NewHandler(tgSvc, logger)

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
