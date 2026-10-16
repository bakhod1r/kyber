# Changelog

All notable changes are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and versions follow [Semantic Versioning](https://semver.org/). Before 1.0, minor versions may change the API;
upgrades are described in [docs/operations/upgrade.md](docs/operations/upgrade.md).

## [0.1.0] — 2026-10-16

The first public release: a self-hosted issue and project tracker with Jira's core, plus team chat and
calls, in one Go binary with a React UI and PostgreSQL. API version 0.76.0 (`api/openapi.yaml`).

### Projects and issues
- Workspaces on subdomains (multi-tenant) or one plain domain; projects, members and roles; permission
  schemes; issue-level security; row-level security in PostgreSQL.
- Issues with keys, sub-tasks, epics and link types; labels; custom fields (text, number, select, date);
  comments with reactions and @mentions; watchers; attachments (local disk or S3); time tracking.
- Data-driven workflows with a visual editor; Kanban and Scrum boards with drag and drop, swimlanes and
  quick filters; backlog and sprints (LexoRank); versions and releases; timeline.
- JQL with autocomplete and database pushdown; saved filters.
- Reports: burndown, velocity, sprint report, control chart, cumulative flow; activity and audit logs.
- Jira CSV import; GitHub integration (commits and pull requests that mention `KYB-12`).

### Collaboration
- Chat (channels, direct messages, threads, bots, incoming webhooks), huddles and calls with screen share,
  recording, a LiveKit media server for large calls, whiteboards, docs, forms, goals, announcements,
  meetings with a time table, and Pomodoro focus.
- Realtime over MQTT with presence across replicas (ADR-0013); in-app, email and Telegram notifications.

### Sign-in and administration
- Password, Google, GitHub, OpenID Connect, SAML 2.0 and Telegram sign-in; personal access tokens; SCIM 2.0
  provisioning.
- Several SSO connections stored in the database with sealed secrets (`KYBER_ENCRYPTION_KEY`), email-domain
  discovery with DNS verification, and per-workspace required SSO (ADR-0018).
- SSO group → role mapping (S106).
- MCP server for AI assistants with OAuth 2.1 (ADR-0016); outgoing webhooks with HMAC signatures (ADR-0014).
- Rate limiting per user (shared through Redis across replicas); administration page; 25 UI languages
  (English, Uzbek and Russian complete), RTL, themes.

### Operations
- Docker image, `docker-compose.yml`, Helm chart; backup/restore and upgrade guides; `/healthz`, `/readyz`,
  Prometheus `/metrics`, OpenTelemetry tracing.
- Forward-only embedded migrations (0001–0093).
- Load test: 10 000 issues per project, reads p95 37.4 ms, 0 % errors (`loadtest/README.md`).
- Release gate `make ship`; internal security review (`docs/operations/security-review-2026-10.md`).

### Known limits
- No external penetration test yet (planned before 1.0).
- SAML connections are configured by environment variables only; stored connections are OpenID Connect.

[0.1.0]: https://github.com/bakhod1r/kyber/releases/tag/v0.1.0
