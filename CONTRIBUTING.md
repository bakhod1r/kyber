# Contributing to Kyber

Thanks for helping! Kyber is a self-hosted issue tracker: Go backend, React + TypeScript web app,
PostgreSQL. `docs/PLAN.md` is the plan and `docs/adr/` records the decisions.

## Ground rules

- **DDD, four layers** (ADR-0005): `domain` (pure), `application`, `infrastructure`, `interfaces`,
  per bounded context, in Go and in the web app. Architecture tests enforce the boundaries.
- **TDD**: write the failing test first. Bug fixes start with a reproducing test. Fakes over mocks.
- **OpenAPI first**: change `api/openapi.yaml` with every endpoint; the contract test fails on drift.
- **SQL is the source of truth**: forward-only migrations in `internal/platform/db/migrations`;
  hand-written pgx repositories that pass the shared contract tests against real PostgreSQL.
- **i18n**: every UI string goes through `web/src/shared/i18n` with en, uz and ru translations.
- **Conventional Commits** (`feat:`, `fix:`, `docs:` …). No secrets in code, logs or fixtures.

## Before you open a pull request

```sh
make dev-db                            # throwaway PostgreSQL 16 on :55432 (Docker, else local binaries)
eval "$(scripts/dev-env.sh env)"       # exports KYBER_TEST_DATABASE_URL
make lint
go test -race ./...
make cover-strict          # packages listed in .cover-strict stay at 100%
make web-test              # Vitest
make e2e                   # Playwright (needs a Chromium)
```

`make dev-db-stop` stops the database. `scripts/dev-env.sh k6` downloads the pinned k6 into `.dev/bin` for the
load test (`loadtest/README.md`).

Fill in the pull request template; keep changes focused, and update the README/ADRs when behaviour
or configuration changes.
