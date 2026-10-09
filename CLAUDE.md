# Kyber

Open-source, self-hosted issue & project tracker (Jira core functionality).
Backend: Go. Frontend: React + TypeScript. DB: PostgreSQL.
Master plan: `docs/PLAN.md`. Decisions: `docs/adr/`.

## Agents

Agents come from the `awesome-agents` marketplace (enabled in `.claude/settings.json`).
Chain: **architect decides → engineer implements → quality verifies → release ships.**

| Work | Agent(s) |
|---|---|
| Scope, backlog, acceptance criteria | product-team |
| Module boundaries, API contracts, ADRs (`/design`) | backend-architect, frontend-architect, database-architect |
| Go services, migrations, queries | backend-team |
| React UI, board, editor | frontend-team, design-team |
| Tests, contract tests, e2e | quality-team |
| AuthZ, threat model (`/threat-model`), audit (`/audit`) | security-team |
| CI, Docker, Helm, observability | platform-team |
| Release gate (`/ship`), versioning | release-team |

Large features: `/flow <goal>`. Every PR: `/review`.

## Method: DDD + TDD (mandatory)

- **DDD**: follow bounded contexts in `docs/PLAN.md` §3.5. Each context = `domain/` (pure, no I/O),
  `app/` (use cases), `adapter/` (postgres, http). Cross-aggregate references by ID only;
  side effects via domain events + outbox. Use the ubiquitous language in code names.
- **TDD**: red → green → refactor. Write the failing test first, run it, see it fail, then
  implement the minimum. Fakes over mocks. Bug fix starts with a reproducing test.
  `go test -race ./...` must pass before every commit.

## Conventions

- Go: `go fmt`, `go vet`, `golangci-lint`; table-driven tests; `internal/` packages; no globals.
- SQL is the source of truth: migrations in `internal/platform/db/migrations` (embedded, forward-only).
- Persistence: hand-written `pgx/v5` repositories in each context's `adapter/postgres`, implementing the
  domain's repository port. **No `sqlc`, no ORM** (ADR-0002). Every adapter passes the shared repository
  contract test (`domain/repotest`) against real PostgreSQL.
- API: OpenAPI 3.1 in `api/openapi.yaml` first; Go server & TS client generated from it.
- Frontend: strict TypeScript, TanStack Query for server state, no `any`.
- Conventional Commits. No secrets in code, logs, or fixtures.
