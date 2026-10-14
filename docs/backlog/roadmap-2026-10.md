# Roadmap (October 2026): classic DDD everywhere, then Jira parity

Every step: plan → failing test → implementation → full verification (`go test -race`, `make cover-strict`,
architecture test, Vitest, e2e ×3) → commit + push → short report.

## A. Backend: finish classic DDD
- A1 Auto-watch through domain events (issue.created / issue.assigned / comment.added → Watch) via the outbox.
- A2 Version release/delete: the version and its event are saved in one transaction; Issue Tracking moves
  fix versions in an event handler (no half-moved state).
- A3 Ubiquitous-language names instead of setters (Classify, PlanForVersions, MoveToVersion).
- A4 ADR-0005 (layering), CLAUDE.md and ADR-0002 updated.

## B. Frontend: DDD by bounded context
- B1 `web/src/contexts/<context>/{domain,application,infrastructure,ui}` + `web/src/shared/` (http, ui kit).
  domain = types and pure rules; application = use cases (TanStack Query hooks); infrastructure = API client
  of that context; ui = components and pages.
- B2 Split `api.ts` per context; move components and tests; no behaviour change.
- B3 Architecture test (Vitest) for the import rules, as on the backend.

## C. Features
1. ✅ Worklog: log work, original/remaining estimate, time tracking in the issue, time statistics in the sprint report.
2. ✅ JQL: lexer/parser in the domain, SQL in infrastructure, memory evaluator, search page, saved filters.
3. ✅ Board settings UI: columns, status mapping, WIP limits.
4. ✅ Custom fields: per-project definitions (text, number, date, select), values on issues, API/UI, JQL.
5. Attachments: storage port; MinIO/S3 and local adapters; size limits, authorization, UI.
   C5 — done: attachment context, local-disk blob store (KYBER_ATTACHMENTS_DIR), 25 MiB limit, issue permissions, safe downloads, issue panel UI (ADR-0011).
   S3/MinIO adapter — done: KYBER_S3_* settings, streaming multipart uploads.
6. S49: event envelope (workspace, project, actor, occurred_at, schema version) and a dead-letter queue. — done (ADR-0007: actor, trace context, schema version; dead letters after 10 failures; `kyber outbox` CLI).
7. Real time (SSE): outbox relay → per-workspace stream filtered by visibility; UI invalidates queries. — done (ADR-0008; hints only, NOTIFY across replicas).
8. Admin panel (workspace users, projects, settings), MCP server (tools over the API), PostgreSQL RLS.
   C8a admin panel — done: members and roles, projects, stuck events (view, retry). 
   C8b MCP server — done: POST /api/v1/mcp with list_projects, search_issues, get_issue, create_issue, add_comment (ADR-0009).
   Personal access tokens — done: kpat_ tokens for scripts and MCP, Settings → Access tokens (ADR-0012).
   C8c PostgreSQL RLS — done: tenant_isolation policy on every workspace table, pool scopes each connection to the request's workspace (ADR-0010).

## D. Jira project navigation (requested 2026-10-13)
Tabs like Jira's: Summary, Timeline, Backlog, Active sprints, Reports, List, Forms, Goals, Docs — plus quick
filters on the board and the list (and saved filters reused there).

Status: D1 Summary, Active sprints, List and quick filters — done. D3 Timeline — done (start/due dates on
issues, `startDate`/`dueDate`/`due` in JQL, the Timeline tab). D4 Goals — done (goal context; progress
by JQL through the Search ACL; Goals tab). D5 Docs — done (docs context;
Markdown pages with numbered revisions; safe Markdown rendering without raw HTML). D6 Forms — done (form context;
submissions become issues through the Issue Tracking ACL). Section D is complete.

## E. Team events and announcements (requested 2026-10-13)
1. E1 Announcements — done: project admins (project) and workspace owners/admins or `KYBER_ADMIN_EMAILS`
   (workspace) publish; every member is notified in the inbox, by email/Telegram per their settings, in
   the project's Telegram groups, and open pages refresh live.
2. E2 Team events — done: meetings for chosen people, a project's members or the workspace's; everyone
   invited is notified when it is scheduled or cancelled (inbox, email/Telegram, the project's Telegram
   groups), reminded once 15 minutes before (once across replicas), and answers going/maybe/declined.
   Rescheduling (moving a meeting) is not there yet.

## F. Remaining plan items (2026-10-14)

Migration numbers are reserved per item so parallel work does not collide.

| # | Item | Scope | Migration |
|---|---|---|---|
| F1 ✅ | MQTT everywhere (ADR-0013) | embedded broker (mochi-mqtt); browser over WebSocket at /api/v1/mqtt; optional TCP listener with PATs; per-message project authorization; replaces SSE | — |
| F2 ✅ | i18n (en/uz/ru) | web UI strings through a typed dictionary; language picker; Accept-Language for API problems | — |
| F3 ✅ | Meetings: reschedule + time zones | PUT /meetings/{id} (re-notifies), user time zone preference, display in it | 0050 |
| F4 | Comment reactions + presence | emoji reactions on comments; who is viewing an issue (over MQTT) | 0051 |
| F5 | Webhooks | per-project outgoing webhooks (HMAC-SHA256 signed, retries via outbox); GitHub commit/PR linking by issue key | 0052 |
| F6 | Generic OIDC + GitHub sign-in | KYBER_OIDC_* (any provider), GitHub OAuth; SAML/SCIM later | — |
| F7 | Board swimlanes | by assignee / epic / none | — |
| F8 ✅ | MCP: more tools + OAuth | transition_issue, assign_issue, list_sprints, get_comments, update_issue; OAuth 2.1 later | — |
| F9 | Sync docs/PLAN.md checkboxes; recheck F5 workflow editor | docs | — |
