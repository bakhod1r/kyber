# ADR-0005: Classic four-layer DDD in every bounded context

- Status: accepted (2026-10-13)
- Supersedes the `app/` + `adapter/` layout described in earlier ADRs and CLAUDE.md.

## Context
Contexts were split into `domain/`, `app/` and `adapter/` (ports and adapters). The owner asked for
classic DDD (Evans): four explicit layers, in the backend and in the frontend.

## Decision
Every bounded context under `internal/<context>/` has exactly four layers:

| Layer | Package | Holds | May depend on |
|---|---|---|---|
| Domain | `domain/` | aggregates, value objects, domain events, repository interfaces | standard library and pure libraries only |
| Application | `application/` | use cases, authorization, ports to other contexts, event handlers | its own domain, `platform/outbox`, `platform/tenant` |
| Infrastructure | `infrastructure/` | pgx and in-memory repositories, anti-corruption layers to other contexts, external services (Google, Telegram, SMTP), schedulers | anything except `interfaces/` |
| Interfaces | `interfaces/` | HTTP handlers and DTOs (the user interface of the API) | its own application and domain, `platform/` |

Rules across contexts:
- Contexts meet only in infrastructure ACLs and in the composition root (`internal/server`).
- References between aggregates and contexts are by id only — no cross-context foreign keys
  (migration 0036 dropped the last one, `issue_fix_versions → versions`).
- Side effects on another aggregate go through domain events saved in the same transaction as the
  aggregate (transactional outbox) and handled by the owning context: auto-watching an issue
  (`issue.created`, `issue.assigned`, `comment.added`) and moving issues when a version is released or
  deleted (`version.released`, `version.deleted`). Handlers are idempotent; the relay retries them.
- Aggregates expose intention-revealing methods (`Classify`, `PlanForVersions`, `MoveToVersion`,
  `ChangeResolution`) instead of setters.

`internal/archtest` enforces the dependency rules on every `go test ./...`.

### Frontend (`web/src`)
The same bounded contexts, the same four layers:

| Layer | Path | Holds | May depend on |
|---|---|---|---|
| Domain | `contexts/<c>/domain/model.ts` | types, constants, pure rules | other contexts' domain models only (published language) |
| Infrastructure | `contexts/<c>/infrastructure/gateway.ts` | the context's HTTP gateway | `shared/http`, domain models |
| Application | `contexts/<c>/application/index.ts` | use cases the UI calls (e.g. `createIssueIn`) | its own gateway, domain models |
| UI | `contexts/<c>/ui/` | components and pages | any context's application, `shared/`; never infrastructure, never another context's UI |

`src/app/` is the composition root (routes, layout, the project page composing every context's view;
cross-context panels are passed in, e.g. `IssuePanel extensions`). `src/shared/` holds the HTTP client,
chart primitives, test utilities and browser-side domain events (`shared/events`: an issue reaching Done
is announced, Focus reacts) and never depends on a context. `src/shared/test/architecture.test.ts`
enforces the rules on every Vitest run.

## Consequences
- Moves across aggregates are eventually consistent (milliseconds with the relay); API responses report
  what *will* move (e.g. a release's `moved`) and tests wait for the projection.
- Sprint completion still returns unfinished issues synchronously before closing the sprint (so a failed
  move leaves the sprint active and the completion can be retried); converting it to events is tracked
  in the roadmap.
