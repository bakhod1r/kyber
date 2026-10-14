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
	// AttachmentsDir holds uploaded files (<dir>/<workspace>/<id>); back it up with the database.
	AttachmentsDir string `env:"KYBER_ATTACHMENTS_DIR" default:"./data/attachments"`
	// S3 stores attachments in an S3-compatible bucket (AWS S3, MinIO) instead of AttachmentsDir
	// when S3Bucket is set. S3Endpoint is host[:port] without a scheme.
	S3Endpoint  string `env:"KYBER_S3_ENDPOINT" default:"s3.amazonaws.com"`
	S3Bucket    string `env:"KYBER_S3_BUCKET"`
	S3Region    string `env:"KYBER_S3_REGION"`
	S3AccessKey string `env:"KYBER_S3_ACCESS_KEY"`
	S3SecretKey string `env:"KYBER_S3_SECRET_KEY"`
	S3TLS       bool   `env:"KYBER_S3_TLS" default:"true"`
	// BaseDomain enables workspaces on subdomains (<slug>.<BaseDomain>, ADR-0004); empty = single-tenant.
	BaseDomain string `env:"KYBER_BASE_DOMAIN"`
	// AdminEmails (comma-separated) administer the default workspace: they publish workspace-wide
	// announcements. Named workspaces have owners and admins of their own.
	AdminEmails string `env:"KYBER_ADMIN_EMAILS"`
	// Sign in with Google: create an OAuth client (Web application) in Google Cloud and add
	// <PublicURL>/api/v1/auth/google/callback as an authorized redirect URI.
	GoogleClientID     string `env:"KYBER_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"KYBER_GOOGLE_CLIENT_SECRET"`
	// Sign in with Telegram: create a bot with @BotFather and /setdomain to the site's domain.
	TelegramBotToken string `env:"KYBER_TELEGRAM_BOT_TOKEN"`
	TelegramBotName  string `env:"KYBER_TELEGRAM_BOT_NAME"`
	// Email notifications through any SMTP server; unset host = no email.
	SMTPHost     string `env:"KYBER_SMTP_HOST"`
	SMTPPort     string `env:"KYBER_SMTP_PORT"`
	SMTPUsername string `env:"KYBER_SMTP_USERNAME"`
	SMTPPassword string `env:"KYBER_SMTP_PASSWORD"`
	SMTPFrom     string `env:"KYBER_SMTP_FROM" default:"Kyber <noreply@localhost>"`
	SMTPTLS      string `env:"KYBER_SMTP_TLS" default:"starttls"` // starttls (587), tls (465) or none
	// Calls (WebRTC): comma-separated stun:/turn:/turns: URLs, and coturn's static-auth-secret for TURN.
	ICEServers string `env:"KYBER_ICE_SERVERS"`
	TURNSecret string `env:"KYBER_TURN_SECRET"`
	// FilesDir keeps chat files and huddle recordings on disk (back it up with the database).
	FilesDir string `env:"KYBER_FILES_DIR"`
	// STTURL turns on transcripts of huddle recordings (S77): an OpenAI-compatible speech-to-text endpoint, e.g.
	// a self-hosted Whisper. Unset (the default), recordings never leave the server.
	STTURL   string `env:"KYBER_STT_URL"`
	STTModel string `env:"KYBER_STT_MODEL"`
	STTKey   string `env:"KYBER_STT_KEY"`
	// LLMURL turns on summaries of those transcripts (S87): an OpenAI-compatible chat completions endpoint, e.g. a
	// self-hosted Ollama. Unset (the default), transcripts never leave the server.
	LLMURL   string `env:"KYBER_LLM_URL"`
	LLMModel string `env:"KYBER_LLM_MODEL"`
	LLMKey   string `env:"KYBER_LLM_KEY"`
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
	return fmt.Sprintf("addr=%s database=%s redis=%s cookie_secure=%t session_purge_every=%s public_url=%s google=%t telegram=%t attachments=%s",
		c.Addr, redact(c.DatabaseURL), redisURL, c.CookieSecure, c.SessionPurgeEvery, c.PublicURL,
		c.GoogleClientID != "" && c.GoogleClientSecret != "", c.TelegramBotToken != "", c.attachments())
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

// attachments names the file store (never its credentials).
func (c Config) attachments() string {
	if c.S3Bucket != "" {
		return "s3://" + c.S3Endpoint + "/" + c.S3Bucket
	}
	return c.AttachmentsDir
}
