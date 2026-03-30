// Slink — URL shortener bot
// Telegram: @SlinkBot
// API: slinkapi

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/evgslyusar/shortlink/internal/config"
	"github.com/evgslyusar/shortlink/internal/repository"
	"github.com/evgslyusar/shortlink/internal/service"
	"github.com/evgslyusar/shortlink/internal/transport"
	mw "github.com/evgslyusar/shortlink/internal/transport/middleware"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer logger.Sync() //nolint:errcheck
	logger = logger.With(zap.String("service", "slinkapi"))

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
	authHandler := transport.NewAuthHandler(authSvc, authSvc, logger)

	// Set up router.
	r := chi.NewRouter()
	r.Use(mw.Correlation)
	r.Use(mw.Recovery(logger))
	r.Use(mw.Logger(logger))

	r.Get("/healthz", handleHealthz())

	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
	})

	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      r,
		ReadTimeout:  cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		logger.Info("slinkapi starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal or server error.
	select {
	case <-ctx.Done():
		logger.Info("shutting down slinkapi")
	case err := <-errCh:
		logger.Error("slinkapi error", zap.Error(err))
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("slinkapi shutdown error", zap.Error(err))
	}
	logger.Info("slinkapi stopped")
}

func handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}
}
