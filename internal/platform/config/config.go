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
	return fmt.Sprintf("addr=%s database=%s redis=%s cookie_secure=%t session_purge_every=%s",
		c.Addr, redact(c.DatabaseURL), redisURL, c.CookieSecure, c.SessionPurgeEvery)
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
