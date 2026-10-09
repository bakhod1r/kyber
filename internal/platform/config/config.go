// Package config loads server settings from the environment (and optional .env files) via oneenv.
package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/bakhod1r/oneenv"
)

type Config struct {
	Addr              string        `env:"KYBER_ADDR" default:":8080"`
	DatabaseURL       string        `env:"KYBER_DATABASE_URL"`
	RedisURL          string        `env:"KYBER_REDIS_URL"`
	CookieSecure      bool          `env:"KYBER_COOKIE_SECURE" default:"true"`
	SessionPurgeEvery time.Duration `env:"KYBER_SESSION_PURGE_EVERY" default:"1h"`
	// PublicURL is the external base URL (OAuth redirect URIs are built from it).
	PublicURL string `env:"KYBER_PUBLIC_URL" default:"http://localhost:8080"`
	// BaseDomain enables workspaces on subdomains (<slug>.<BaseDomain>, ADR-0004); empty = single-tenant.
	BaseDomain string `env:"KYBER_BASE_DOMAIN"`
	// Sign in with Google: create an OAuth client (Web application) in Google Cloud and add
	// <PublicURL>/api/v1/auth/google/callback as an authorized redirect URI.
	GoogleClientID     string `env:"KYBER_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"KYBER_GOOGLE_CLIENT_SECRET"`
	// Sign in with Telegram: create a bot with @BotFather and /setdomain to the site's domain.
	TelegramBotToken string `env:"KYBER_TELEGRAM_BOT_TOKEN"`
	TelegramBotName  string `env:"KYBER_TELEGRAM_BOT_NAME"`
}

func Load() (*Config, error) {
	cfg, err := oneenv.Parse[Config]()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

// String is safe to log: the database password is redacted.
func (c Config) String() string {
	redisURL := "(none: per-process login limiter)"
	if c.RedisURL != "" {
		redisURL = redact(c.RedisURL)
	}
	return fmt.Sprintf("addr=%s database=%s redis=%s cookie_secure=%t session_purge_every=%s public_url=%s google=%t telegram=%t",
		c.Addr, redact(c.DatabaseURL), redisURL, c.CookieSecure, c.SessionPurgeEvery, c.PublicURL,
		c.GoogleClientID != "" && c.GoogleClientSecret != "", c.TelegramBotToken != "")
}

func redact(raw string) string {
	if raw == "" {
		return "(in-memory)"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "(unparseable)"
	}
	return u.Redacted()
}
