// Command kyber runs the Kyber server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := os.Getenv("KYBER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler, cleanup, err := newHandler(ctx, log)
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
func newHandler(ctx context.Context, log *slog.Logger) (http.Handler, func(), error) {
	url := os.Getenv("KYBER_DATABASE_URL")
	if url == "" {
		log.Warn("KYBER_DATABASE_URL not set: running in-memory dev mode, data is lost on restart")
		return server.NewInMemory(log), func() {}, nil
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
	server.StartJobs(ctx, log, pool)
	secure := os.Getenv("KYBER_COOKIE_SECURE") != "false"
	return server.NewPostgres(log, pool, secure), pool.Close, nil
}
