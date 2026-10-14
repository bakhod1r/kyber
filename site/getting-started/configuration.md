# Configuration reference

Kyber reads its settings from the environment (and optional `.env` files). Every variable below is
defined in
[`internal/platform/config/config.go`](https://github.com/bakhod1r/kyber/blob/main/internal/platform/config/config.go);
defaults are the ones declared there. Keep secrets (passwords, tokens, client secrets) in the environment
or a secret store, never in committed files.

## Core

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_ADDR` | `:8080` | Listen address |
| `KYBER_DATABASE_URL` | — | PostgreSQL URL; unset = in-memory dev mode. Connect as a non-superuser role so row-level security (ADR-0010) applies |
| `KYBER_REDIS_URL` | — | Optional; shares the login limiter and the API rate-limit budget across replicas |
| `KYBER_PUBLIC_URL` | `http://localhost:8080` | External base URL; OAuth redirect URIs and OAuth metadata are built from it |
| `KYBER_COOKIE_SECURE` | `true` | Set `false` only for plain-HTTP local use |
| `KYBER_SESSION_PURGE_EVERY` | `1h` | Expired-session cleanup interval (jittered) |
| `KYBER_BASE_DOMAIN` | — | Enables workspaces on subdomains (`<slug>.<base domain>`, ADR-0004); empty = single-tenant |
| `KYBER_ADMIN_EMAILS` | — | Comma-separated emails administering the default workspace (single-tenant mode): they publish workspace-wide announcements |

## Files

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_ATTACHMENTS_DIR` | `./data/attachments` | Uploaded files (`<dir>/<workspace>/<id>`, at most 25 MiB each); back it up together with the database |
| `KYBER_S3_BUCKET` | — | Store attachments in an existing S3/MinIO bucket instead of the directory |
| `KYBER_S3_ENDPOINT` | `s3.amazonaws.com` | `host[:port]`, without a scheme |
| `KYBER_S3_REGION` | — | Bucket region |
| `KYBER_S3_ACCESS_KEY` / `KYBER_S3_SECRET_KEY` | — | Credentials |
| `KYBER_S3_TLS` | `true` | Use HTTPS to reach the endpoint |
| `KYBER_FILES_DIR` | — | Chat files and huddle recordings on disk; unset = in memory (lost on restart). With `KYBER_S3_BUCKET` they go to the bucket under `chat/` |

## Rate limits

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_API_RATE_LIMIT` | `1200` | API requests per signed-in user per minute; 0 = off. See [Rate limiting](../operations/rate-limiting) |
| `KYBER_UPLOAD_RATE_LIMIT` | `60` | Attachment/file uploads per user per minute; 0 = off |

## Integrations

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_MQTT_ADDR` | — | Also serve plain MQTT (e.g. `:1883`); username = workspace id, password = access token. See [Live updates](../guide/live-updates) |
| `KYBER_GITHUB_WEBHOOK_SECRET` | — | Enables `POST /api/v1/integrations/github`. See [Webhooks & GitHub](../guide/webhooks) |
| `KYBER_WEBHOOK_ALLOW_PRIVATE` | `false` | Let project webhooks reach loopback/private/link-local addresses (refused by default against SSRF) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Enables OpenTelemetry tracing (OTLP/HTTP, e.g. `http://otel-collector:4318`); the standard `OTEL_*` variables apply (ADR-0006) |

## Sign-in

See [Single sign-on & SCIM](../guide/sso) for setup steps.

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_GOOGLE_CLIENT_ID` / `KYBER_GOOGLE_CLIENT_SECRET` | — | "Continue with Google" |
| `KYBER_OIDC_ISSUER` / `KYBER_OIDC_CLIENT_ID` / `KYBER_OIDC_CLIENT_SECRET` | — | Single sign-on with any OpenID Connect provider |
| `KYBER_OIDC_NAME` | `SSO` | Label of the OIDC button ("Continue with …") |
| `KYBER_GITHUB_CLIENT_ID` / `KYBER_GITHUB_CLIENT_SECRET` | — | "Continue with GitHub" |
| `KYBER_SAML_IDP_METADATA_URL` / `KYBER_SAML_IDP_METADATA_FILE` | — | Enables "Continue with SAML" |
| `KYBER_SAML_CERT_FILE` / `KYBER_SAML_KEY_FILE` | — | The service provider's RSA certificate and key (PEM) |
| `KYBER_SAML_NAME` | `SAML` | Label of the SAML button |
| `KYBER_SCIM_TOKEN` | — | Enables SCIM 2.0 provisioning at `/scim/v2` (Bearer token) |
| `KYBER_TELEGRAM_BOT_TOKEN` / `KYBER_TELEGRAM_BOT_NAME` | — | Telegram sign-in, one-time login codes and Telegram notifications (run one replica with the token: Kyber long-polls the bot) |

## Email

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_SMTP_HOST` | — | SMTP server; unset = no email |
| `KYBER_SMTP_PORT` | by TLS mode | Port |
| `KYBER_SMTP_USERNAME` / `KYBER_SMTP_PASSWORD` | — | Credentials |
| `KYBER_SMTP_FROM` | `Kyber <noreply@localhost>` | Sender |
| `KYBER_SMTP_TLS` | `starttls` | `starttls` (587), `tls` (465) or `none` (local relay) |

Failed sends are retried with backoff for about 2½ hours.

## Calls, transcripts and summaries

| Variable | Default | Purpose |
|---|---|---|
| `KYBER_ICE_SERVERS` | — | Comma-separated `stun:` / `turn:` / `turns:` URLs for huddles |
| `KYBER_TURN_SECRET` | — | coturn's `static-auth-secret`; Kyber hands browsers short-lived TURN credentials |
| `KYBER_STT_URL` / `KYBER_STT_MODEL` / `KYBER_STT_KEY` | — | OpenAI-compatible speech-to-text endpoint for huddle recording transcripts; unset = recordings never leave the server |
| `KYBER_LLM_URL` / `KYBER_LLM_MODEL` / `KYBER_LLM_KEY` | — | OpenAI-compatible chat completions endpoint for transcript summaries; unset = transcripts never leave the server |

## Operator commands

- `kyber outbox dead-letters` lists outbox events parked after 10 failed deliveries.
- `kyber outbox retry <id>` queues one again (needs `KYBER_DATABASE_URL`; ADR-0007).
