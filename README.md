<p align="center"><img src="docs/assets/logo.svg" width="120" alt="Kyber"></p>
<h1 align="center">Kyber</h1>
<p align="center">Open-source, self-hosted issue &amp; project tracker — the core of Jira, in one Go binary.</p>

> Status: **v0.3 (Sprint 03)** — web UI with Kanban board, project roles, PostgreSQL, accounts, throttled login, outbox, metrics.

<p align="center"><img src="docs/assets/board.png" alt="Kyber Kanban board" width="820"></p>
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
| `KYBER_COOKIE_SECURE` | `true` | Set `false` only for plain-HTTP local use |

## API (v1)

| Method | Path | Description |
|---|---|---|
| GET | `/healthz` · `/readyz` · `/metrics` | Liveness · readiness (DB) · Prometheus |
| POST | `/api/v1/auth/signup` · `/auth/login` · `/auth/logout` | Accounts & sessions (cookie or Bearer) |
| GET | `/api/v1/me` | Current user |
| GET / POST | `/api/v1/projects/{key}/members` | List members / add or change a member's role (admin) |
| POST / GET | `/api/v1/projects` | Create / list projects |
| GET | `/api/v1/projects/{key}` | Get project |
| POST / GET | `/api/v1/projects/{key}/issues[?status=]` | Create / list issues |
| GET | `/api/v1/issues/{KEY-N}` | Get issue |
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

## License

MIT — see [LICENSE](LICENSE).
