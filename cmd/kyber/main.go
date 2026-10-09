// Command kyber runs the Kyber server.
package main

import (
	"strings"

	"context"
	"errors"
	"fmt"
	"github.com/bakhod1r/kyber/internal/identity/adapter/google"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/adapter/httpapi"
	"github.com/bakhod1r/kyber/internal/identity/adapter/telegram"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/bakhod1r/kyber/internal/platform/config"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	log.Info("configuration loaded", "config", cfg.String())
	addr := cfg.Addr
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler, cleanup, err := newHandler(ctx, log, cfg)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer cleanup()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

// newHandler selects Postgres when KYBER_DATABASE_URL is set, otherwise in-memory dev mode.
func newHandler(ctx context.Context, log *slog.Logger, cfg *config.Config) (http.Handler, func(), error) {
	social := server.WithSocial(identityhttp.Social{
		Google: google.New(google.Config{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret,
			RedirectURL: strings.TrimRight(cfg.PublicURL, "/") + "/api/v1/auth/google/callback"}),
		Telegram:    telegram.NewVerifier(cfg.TelegramBotToken, time.Now),
		TelegramBot: cfg.TelegramBotName,
	})
	url := cfg.DatabaseURL
	if url == "" {
		log.Warn("KYBER_DATABASE_URL not set: running in-memory dev mode, data is lost on restart")
		return server.NewInMemory(log, social), func() {}, nil
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	if err := db.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, err
	}
	log.Info("database ready")
	server.StartJobs(ctx, log, pool, cfg.SessionPurgeEvery)
	opts := []server.Option{social}
	cleanup := pool.Close
	if cfg.RedisURL != "" {
		ropts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("parse KYBER_REDIS_URL: %w", err)
		}
		rdb := redis.NewClient(ropts)
		if err := rdb.Ping(ctx).Err(); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("redis: %w", err)
		}
		log.Info("redis ready: login limiter shared across instances")
		opts = append(opts, server.WithRedis(rdb))
		cleanup = func() { _ = rdb.Close(); pool.Close() }
	} else {
		log.Warn("KYBER_REDIS_URL not set: login limiter is per process (fine for a single instance)")
	}
	return server.NewPostgres(log, pool, cfg.CookieSecure, opts...), cleanup, nil
}
