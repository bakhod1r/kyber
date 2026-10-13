<p align="center"><img src="docs/assets/logo-full.png" width="200" alt="Kyber — lightsaber K-crystal logo"></p>
<h1 align="center">Kyber</h1>
<p align="center">Open-source, self-hosted issue &amp; project tracker — the core of Jira, in one Go binary.</p>

> Status: **v0.10 (Sprint 10)** — configurable workflows per project (statuses with To Do / In Progress / Done
> categories), Scrum & Kanban boards with status-mapped columns and WIP limits, backlog & sprints, reports (burndown,
> velocity, created vs resolved, cycle time, workload), notifications & @mentions, Pomodoro focus with a meetings
> time table, eye-break exercises, Jira CSV import/export, Google & Telegram sign-in, multi-tenant workspaces on
> subdomains, OpenAPI contract, localized errors (en/uz/ru), PostgreSQL, optional Redis.

<p align="center"><img src="docs/assets/landing.png" alt="Kyber landing page" width="820"></p>

| | |
|---|---|
| <img src="docs/assets/board.png" alt="Board with columns from the project workflow, including Code Review"> | <img src="docs/assets/workflow.png" alt="Workflow editor: statuses and their categories"> |
| **Board** — columns follow the project's workflow; drag cards between statuses. | **Workflow** — add, rename and remove statuses; each has a Jira category. |
| <img src="docs/assets/backlog.png" alt="Backlog with an active and a planned sprint"> | <img src="docs/assets/reports.png" alt="Reports: KPIs, issues by status, type, priority and workload"> |
| **Backlog & sprints** — rank issues, plan, start and complete sprints. | **Reports** — KPIs, distributions, burndown, velocity, cycle time. |
| <img src="docs/assets/issue-panel.png" alt="Issue panel with focus timer and comments"> | <img src="docs/assets/meetings.png" alt="Meetings time table"> |
| **Issue** — details, Pomodoro focus, comments with @mentions. | **Time table** — no Pomodoro can start during a meeting. |
| <img src="docs/assets/notifications.png" alt="Notifications for mentions and comments"> | |
| **Notifications** — comments and mentions, per workspace. | |

Screenshots are generated from a seeded demo with `make screenshots`.

> Roadmap: [`docs/PLAN.md`](docs/PLAN.md) · Backlog: [`docs/backlog`](docs/backlog) · QA: [`docs/qa`](docs/qa)

## Quick start

```bash
docker compose up --build    # PostgreSQL + Kyber (UI + API) on http://localhost:8080

curl -XPOST localhost:8080/api/v1/auth/signup -d '{"email":"me@example.com","name":"Me","password":"a long password"}'
TOKEN=$(curl -s -XPOST localhost:8080/api/v1/auth/login -d '{"email":"me@example.com","password":"a long password"}' | jq -r .token)
H="Authorization: Bearer $TOKEN"
curl -H "$H" -XPOST localhost:8080/api/v1/projects -d '{"key":"KYB","name":"Kyber"}'
curl -H "$H" -XPOST localhost:8080/api/v1/projects/KYB/issues -d '{"title":"Login page","type":"task"}'
curl -H "$H" -XPOST localhost:8080/api/v1/issues/KYB-1/transitions -d '{"to":"in_progress"}'
```

| Env | Default | Purpose |
|---|---|---|
| `KYBER_ADDR` | `:8080` | Listen address |
| `KYBER_DATABASE_URL` | — | PostgreSQL URL; unset = in-memory dev mode |
| `KYBER_REDIS_URL` | — | Optional; shares the login limiter across replicas (guard) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | Enables OpenTelemetry tracing (OTLP/HTTP, e.g. `http://otel-collector:4318`); unset = tracing off. The standard `OTEL_*` variables apply (`OTEL_EXPORTER_OTLP_HEADERS`, `OTEL_TRACES_SAMPLER`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_SDK_DISABLED`). See ADR-0006 |
| — | — | Operators: `kyber outbox dead-letters` lists outbox events parked after 10 failed deliveries, `kyber outbox retry <id>` queues one again (needs `KYBER_DATABASE_URL`; ADR-0007) |
| `KYBER_PUBLIC_URL` | `http://localhost:8080` | External base URL; OAuth redirect URIs are built from it |
| `KYBER_GOOGLE_CLIENT_ID` / `KYBER_GOOGLE_CLIENT_SECRET` | — | Enables "Continue with Google". Google Cloud → APIs & Services → Credentials → OAuth client (Web); authorized redirect URI `<KYBER_PUBLIC_URL>/api/v1/auth/google/callback` |
| `KYBER_TELEGRAM_BOT_TOKEN` / `KYBER_TELEGRAM_BOT_NAME` | — | Enables Telegram sign-in. Create a bot with @BotFather. With both set the bot also sends **one-time login codes** (works on any domain; Kyber long-polls the bot, so run one replica with the token). The Login Widget additionally needs `/setdomain` to your site's domain. Users can also **connect Telegram for notifications** (Notifications page → Connect Telegram → press Start in the bot), and project admins can **connect Telegram groups** to a project (project → Notifications → Connect a Telegram group) |
| `KYBER_SMTP_HOST` / `_PORT` / `_USERNAME` / `_PASSWORD` / `_FROM` / `_TLS` | — / by TLS mode / — / — / `Kyber <noreply@localhost>` / `starttls` | Email notifications through any SMTP server (Gmail, Mailgun, Postfix…). `_TLS`: `starttls` (port 587), `tls` (465) or `none` (local relay). Failed sends are retried with backoff for about 2½ hours |
| `KYBER_COOKIE_SECURE` | `true` | Set `false` only for plain-HTTP local use |
| `KYBER_SESSION_PURGE_EVERY` | `1h` | Expired-session cleanup interval (jittered) |

## API (v1)

Full contract: [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.1, enforced by the test suite).
Errors are [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) `application/problem+json` with a stable `code`; titles follow `Accept-Language` (en, uz, ru).

| Method | Path | Description |
|---|---|---|
| GET | `/healthz` · `/readyz` · `/metrics` | Liveness · readiness (DB) · Prometheus |
| POST | `/api/v1/auth/signup` · `/auth/login` · `/auth/logout` | Accounts & sessions (cookie or Bearer) |
| GET | `/api/v1/me` | Current user |
| GET / POST | `/api/v1/projects/{key}/members` | List members / add or change a member's role (admin) |
| POST / GET | `/api/v1/projects` | Create / list projects |
| GET | `/api/v1/projects/{key}` | Get project |
| POST / GET | `/api/v1/projects/{key}/issues[?status=]` | Create / list issues |
| GET · PATCH | `/api/v1/issues/{KEY-N}` | Get issue · edit title/description/priority/assignee (with `version`) |
| GET · POST | `/api/v1/issues/{KEY-N}/comments` | List · add comments |
| POST | `/api/v1/issues/{KEY-N}/rank` | Move in the backlog (`after` / `before` another issue) |
| GET · POST | `/api/v1/projects/{key}/sprints` | List · create sprints |
| POST | `/api/v1/sprints/{id}/start` · `/complete` | Start · complete (unfinished issues return to the backlog) |
| GET | `/api/v1/projects/{key}/reports/summary` · `/created-vs-resolved?days=` · `/velocity` | KPIs & distributions · daily flow · committed vs completed |
| GET | `/api/v1/sprints/{id}/burndown` | Remaining story points (step line) + ideal line |
| GET | `/api/v1/notifications[?unread=true]` | Your notifications (assigned, commented, `@email` mentioned) + unread count |
| POST | `/api/v1/notifications/{id}/read` · `/read-all` | Mark one · all as read |
| POST | `/api/v1/issues/{KEY-N}/transitions` | Move issue (`todo ⇄ in_progress ⇄ done`) |

## Development

Domain-Driven Design + Test-Driven Development — see [`CLAUDE.md`](CLAUDE.md) and `docs/PLAN.md` §3.5–3.6.

Roles: `admin` (manage members) ⊃ `member` (create/move issues) ⊃ `viewer` (read). Non-members get `404`.

```bash
make web          # build the React UI (embedded into the Go binary)
make lint test build
make web-test     # typecheck + Vitest
make e2e          # Playwright against a fresh server
KYBER_TEST_DATABASE_URL=postgres://… make test   # + PostgreSQL integration tests
```

## Built with

Kyber reuses the author's libraries: [emailx](https://github.com/bakhod1r/emailx), [errorx](https://github.com/bakhod1r/errorx),
[guard](https://github.com/bakhod1r/guard), [oneenv](https://github.com/bakhod1r/oneenv), [jitterx](https://github.com/bakhod1r/jitterx),
[ctxsentinel](https://github.com/bakhod1r/ctxsentinel) — plus pgx, React and TanStack Query.

## License

MIT — see [LICENSE](LICENSE).
