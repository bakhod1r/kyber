# DDD contexts

Kyber is a **modular monolith**: one Go binary containing clearly separated bounded contexts, built with
Domain-Driven Design and Test-Driven Development.

## Four layers in every context

Each context under `internal/<context>/` has exactly four layers
([ADR-0005](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0005-classic-ddd-layers.md)):

| Layer | Package | Holds |
|---|---|---|
| Domain | `domain/` | Aggregates, value objects, domain events, repository interfaces — pure, no I/O |
| Application | `application/` | Use cases, authorization, ports to other contexts, event handlers |
| Infrastructure | `infrastructure/` | pgx and in-memory repositories, anti-corruption layers, external services |
| Interfaces | `interfaces/` | HTTP handlers and DTOs |

Rules, enforced by `internal/archtest`:

- Contexts meet only in infrastructure ACLs and in the composition root (`internal/server`).
- Aggregates and contexts reference each other by id only — no cross-context foreign keys.
- Side effects run from domain events through a transactional **outbox**
  ([ADR-0007](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0007-event-envelope-and-dead-letters.md)).
- Repositories are hand-written `pgx/v5` code that passes a shared contract test against real PostgreSQL
  ([ADR-0002](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0002-persistence-pgx-without-sqlc.md)).

The React frontend mirrors the same contexts and layers under `web/src/contexts/<context>/`.

## Core contexts

| Context | Package | Role |
|---|---|---|
| Identity | `internal/identity` | Users, sessions, tokens, SSO, SCIM |
| Workspace | `internal/workspace` | Tenants and memberships |
| Access | `internal/access` | Permission schemes and attribute rules |
| Project | `internal/project` | Projects, members, roles |
| Issue Tracking | `internal/issue` | Issues, comments, links, labels, workflows |
| Agile | `internal/agile` | Boards, sprints, backlog rank |
| Search | `internal/search` | JQL and saved filters |
| Insights | `internal/insights` | Reports |
| Notify | `internal/notify` | Notifications, email, Telegram |
| Attachment | `internal/attachment` | Files on disk or S3 |
| Webhook | `internal/webhook` | Outgoing webhooks, GitHub links |

Further contexts: `admin`, `announcement`, `audit`, `calendar`, `call`, `chat`, `docs`, `focus`, `form`, `goal`,
`importer`, `whiteboard`. Shared infrastructure (config, database, outbox, MQTT, MCP, tracing) lives in
`internal/platform`.

See the [decision records](./adr) for the reasoning behind each choice.
