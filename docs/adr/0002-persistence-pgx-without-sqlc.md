# ADR-0002 — Persistence: hand-written pgx repositories behind DDD ports (no sqlc, no ORM)

- **Status:** Accepted (2026-10-09, project owner decision)
- **Supersedes:** the `pgx + sqlc` proposal in `docs/PLAN.md` §6

## Context
Each bounded context owns a repository **port** in `domain/` (e.g. `issue/domain.Repository`). Aggregates
are rebuilt via `Rehydrate` and persisted with optimistic locking, row locks and a transactional outbox.
Code generation (`sqlc`) would add a build step and generated structs that sit between SQL and the domain,
which the repositories would immediately map away again.

## Decision
- Repositories are written by hand with `pgx/v5` in `<context>/adapter/postgres`.
- SQL lives next to the adapter that uses it; schema changes only via embedded, forward-only migrations.
- Correctness is enforced by tests, not codegen: every adapter must pass the context's shared
  **repository contract suite** (memory and Postgres run the same tests) plus Postgres integration tests in CI.
- No ORM, no `sqlc`.

## Consequences
- ✅ The domain stays pure; adapters translate rows ↔ aggregates explicitly (`Rehydrate`).
- ✅ No generator in the toolchain; transactions, `FOR UPDATE`, `ON CONFLICT` and outbox writes are explicit.
- ⚠️ Column/type drift is caught by integration tests rather than at compile time — the contract suite
  and `KYBER_TEST_DATABASE_URL` CI job are therefore mandatory, not optional.
