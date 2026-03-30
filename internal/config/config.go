package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all application configuration loaded from environment variables.
// The application fails fast at startup if required values are missing.
type Config struct {
	ServerHost string `env:"SERVER_HOST" envDefault:"0.0.0.0"`
	ServerPort int    `env:"SERVER_PORT" envDefault:"8080"`
	BaseURL    string `env:"BASE_URL"    envDefault:"http://localhost:8080"`

	DatabaseURL string `env:"DATABASE_URL,required"`
	RedisURL    string `env:"REDIS_URL,required"`

	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`

	JWTPrivateKeyPath string        `env:"JWT_PRIVATE_KEY_PATH,required"`
	JWTPublicKeyPath  string        `env:"JWT_PUBLIC_KEY_PATH,required"`
	JWTAccessTTL      time.Duration `env:"JWT_ACCESS_TTL"  envDefault:"15m"`
	JWTRefreshTTL     time.Duration `env:"JWT_REFRESH_TTL" envDefault:"168h"`

	TelegramBotToken   string `env:"TELEGRAM_BOT_TOKEN,required"`
	TelegramWebhookURL string `env:"TELEGRAM_WEBHOOK_URL,required"`
	BotHost            string `env:"BOT_HOST" envDefault:"0.0.0.0"`
	BotPort            int    `env:"BOT_PORT" envDefault:"8081"`
}

// Addr returns the host:port string for the HTTP server listener.
func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
}

// Load reads configuration from environment variables and validates required fields.
func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
