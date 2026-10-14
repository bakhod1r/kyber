// Command kyber runs the Kyber server.
package main

import (
	"github.com/bakhod1r/kyber/internal/attachment/infrastructure/blobs3"
	"runtime/debug"
	"strings"

	"context"
	"errors"
	"fmt"
	identitydomain "github.com/bakhod1r/kyber/internal/identity/domain"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/github"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/google"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/oidcprovider"
	"github.com/bakhod1r/kyber/internal/identity/infrastructure/telegram"
	identityhttp "github.com/bakhod1r/kyber/internal/identity/interfaces/httpapi"
	"github.com/bakhod1r/kyber/internal/notify/infrastructure/smtpmail"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // users' time zones work in images without /usr/share/zoneinfo

	"github.com/redis/go-redis/v9"

	"github.com/bakhod1r/kyber/internal/call/infrastructure/ice"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/llm"
	"github.com/bakhod1r/kyber/internal/chat/infrastructure/stt"
	blobs3chat "github.com/bakhod1r/kyber/internal/platform/blob/s3"
	"github.com/bakhod1r/kyber/internal/platform/config"
	"github.com/bakhod1r/kyber/internal/platform/db"
	"github.com/bakhod1r/kyber/internal/platform/otel"
	"github.com/bakhod1r/kyber/internal/platform/outbox"
	"github.com/bakhod1r/kyber/internal/server"
)

func main() {
	log := slog.New(otel.LogHandler{Handler: slog.NewJSONHandler(os.Stdout, nil)})
	cfg, err := config.Load()
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "outbox" {
		os.Exit(outboxCommand(log, cfg, os.Args[2:]))
	}
	log.Info("configuration loaded", "config", cfg.String())
	addr := cfg.Addr
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownTracing, err := otel.Setup(ctx, "kyber", buildVersion(), os.Getenv)
	if err != nil {
		log.Error("tracing setup failed", "err", err)
		os.Exit(1)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			log.Warn("tracing shutdown", "err", err)
		}
	}()
	log.Info("tracing", "enabled", otel.Enabled(os.Getenv))

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
	callback := func(name string) string {
		return strings.TrimRight(cfg.PublicURL, "/") + "/api/v1/auth/" + name + "/callback"
	}
	social := server.WithSocial(identityhttp.Social{
		Google: google.New(google.Config{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret, RedirectURL: callback("google")}),
		OIDC: oidcprovider.New(oidcprovider.Config{Provider: identitydomain.ProviderOIDC, Issuer: cfg.OIDCIssuer, ClientID: cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret, RedirectURL: callback("oidc"), RequireVerifiedEmail: true}),
		OIDCName:    cfg.OIDCName,
		GitHub:      github.New(github.Config{ClientID: cfg.GitHubClientID, ClientSecret: cfg.GitHubClientSecret, RedirectURL: callback("github")}),
		Telegram:    telegram.NewVerifier(cfg.TelegramBotToken, time.Now),
		TelegramBot: cfg.TelegramBotName,
	})
	common := append([]server.Option{social, server.WithContext(ctx), server.WithBaseDomain(cfg.BaseDomain), server.WithSiteAdmins(cfg.AdminEmails),
		server.WithPublicURL(cfg.PublicURL), server.WithAttachmentsDir(cfg.AttachmentsDir), server.WithMQTTAddr(cfg.MQTTAddr),
		server.WithICE(ice.Parse(cfg.ICEServers), cfg.TURNSecret),
		server.WithWebhooks(cfg.WebhookAllowPrivate, 0), server.WithGitHubSecret(cfg.GitHubWebhookSecret),
		server.WithRateLimits(cfg.APIRateLimit, cfg.UploadRateLimit)}, telegramBot(cfg)...)
	if cfg.S3Bucket != "" {
		blobs, err := blobs3.New(ctx, blobs3.Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, Region: cfg.S3Region,
			AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, TLS: cfg.S3TLS})
		if err != nil {
			return nil, nil, err
		}
		common = append(common, server.WithAttachmentStore(blobs))
		chat, err := blobs3chat.New(ctx, blobs3chat.Config{Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, Region: cfg.S3Region,
			AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, TLS: cfg.S3TLS, Prefix: "chat/"})
		if err != nil {
			return nil, nil, err
		}
		common = append(common, server.WithChatFiles(chat)) // chat files and recordings too (S82)
	}
	if cfg.FilesDir != "" {
		common = append(common, server.WithFilesDir(cfg.FilesDir))
	}
	if cfg.STTURL != "" {
		common = append(common, server.WithTranscriber(stt.New(cfg.STTURL, cfg.STTModel, cfg.STTKey)))
	}
	if cfg.LLMURL != "" {
		common = append(common, server.WithSummarizer(llm.New(cfg.LLMURL, cfg.LLMModel, cfg.LLMKey)))
	}
	mailer, err := mailer(cfg)
	if err != nil {
		return nil, nil, err
	}
	common = append(common, mailer...)
	url := cfg.DatabaseURL
	if url == "" {
		log.Warn("KYBER_DATABASE_URL not set: running in-memory dev mode, data is lost on restart")
		return server.NewInMemory(log, common...), func() {}, nil
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
	opts := common
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

// telegramBot enables login codes from the bot when a bot token is configured.
func telegramBot(cfg *config.Config) []server.Option {
	if cfg.TelegramBotToken == "" || cfg.TelegramBotName == "" {
		return nil
	}
	return []server.Option{server.WithTelegramBot(telegram.NewBot(cfg.TelegramBotToken, ""), 50*time.Second)}
}

// mailer enables email notifications when an SMTP host is configured.
func mailer(cfg *config.Config) ([]server.Option, error) {
	if cfg.SMTPHost == "" {
		return nil, nil
	}
	m, err := smtpmail.New(smtpmail.Config{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword, From: cfg.SMTPFrom, TLS: cfg.SMTPTLS})
	if err != nil {
		return nil, err
	}
	return []server.Option{server.WithMailer(m)}, nil
}

// buildVersion is the module version the binary was built from ("(devel)" for local builds),
// or its VCS revision when that is known.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, kv := range info.Settings {
		if kv.Key == "vcs.revision" {
			return kv.Value
		}
	}
	return info.Main.Version
}

// outboxCommand runs `kyber outbox …` against KYBER_DATABASE_URL and returns the exit code.
func outboxCommand(log *slog.Logger, cfg *config.Config, args []string) int {
	if cfg.DatabaseURL == "" {
		log.Error("kyber outbox needs KYBER_DATABASE_URL")
		return 2
	}
	ctx := context.Background()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		return 1
	}
	defer pool.Close()
	if err := runOutbox(ctx, args, outbox.NewPostgresStore(pool), os.Stdout); err != nil {
		log.Error("kyber outbox", "err", err)
		return 1
	}
	return 0
}
