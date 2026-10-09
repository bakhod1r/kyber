package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/config"
)

func TestDefaults(t *testing.T) {
	t.Setenv("KYBER_DATABASE_URL", "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || !cfg.CookieSecure || cfg.DatabaseURL != "" || cfg.SessionPurgeEvery != time.Hour {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestOverrides(t *testing.T) {
	t.Setenv("KYBER_ADDR", ":9000")
	t.Setenv("KYBER_DATABASE_URL", "postgres://u:secret@db/kyber")
	t.Setenv("KYBER_COOKIE_SECURE", "false")
	t.Setenv("KYBER_SESSION_PURGE_EVERY", "15m")
	t.Setenv("KYBER_REDIS_URL", "redis://:topsecret@cache:6379/0")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9000" || cfg.CookieSecure || cfg.DatabaseURL != "postgres://u:secret@db/kyber" || cfg.SessionPurgeEvery != 15*time.Minute {
		t.Fatalf("cfg = %+v", cfg)
	}
	// The database URL carries credentials and must never be printed.
	if cfg.RedisURL != "redis://:topsecret@cache:6379/0" {
		t.Fatalf("redis url = %q", cfg.RedisURL)
	}
	if s := cfg.String(); strings.Contains(s, "secret") {
		t.Fatalf("String() leaks the database password: %s", s)
	}
}

func TestInvalid(t *testing.T) {
	t.Setenv("KYBER_COOKIE_SECURE", "maybe")
	if _, err := config.Load(); err == nil {
		t.Fatal("invalid bool must fail")
	}
}
