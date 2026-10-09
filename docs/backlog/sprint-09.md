# Sprint 09 — Multi-tenant workspaces, attachments (MinIO), realtime (SSE)

Product owner priorities, in order. Each story ships DDD + TDD with acceptance tests on all backends.

## S33 — Workspaces (multi-tenant)

One Kyber installation hosts many organizations; their data never mixes.

- New **Workspace** bounded context: workspace (slug, name), members with roles owner/admin/member, invitations.
- Every project belongs to exactly one workspace; project keys are unique per workspace, not globally.
- Tenant isolation: every table carrying tenant data gets `workspace_id NOT NULL`; every repository query
  filters by it; the workspace is resolved from the URL (`/api/v1/w/{slug}/...`) and checked against membership.
- PostgreSQL Row-Level Security as a second line of defence (`SET LOCAL kyber.workspace_id`), decided in ADR.
- Migration: existing data moves into a "default" workspace owned by the existing project admins.
- Acceptance: a user of workspace A gets 404 for every URL of workspace B (contract-tested across all endpoints).

## S34 — Attachments on MinIO (S3 API)

- Upload/download/delete files on an issue; stored in MinIO (any S3-compatible store) behind a `BlobStore` port.
- Direct-to-storage presigned uploads, 25 MB limit, content-type sniffing, image thumbnails.
- `docker compose` gains a MinIO service; memory blob store for tests.

## S35 — Realtime board updates over SSE

- `GET /api/v1/projects/{key}/events` streams Server-Sent Events fed by the outbox relay.
- Board, backlog and issue panel update without reload; reconnect with `Last-Event-ID` resumes from the outbox id.
- Works behind proxies (heartbeat comments every 15 s); per-user authorization on every event.

## Carry-over from Sprint 08

S30 CSV export, S31 @mention autocomplete, S32 MCP server (needs personal API tokens).

## S36 — Public landing page ✅ (shipped)
Visitors get a landing page at `/`; signed-in users get their projects.

## S37 — Superuser admin panel
Platform operators (super admins) manage the whole installation:
- Users: list/search, suspend/reactivate, reset sessions, promote to super admin (guard's protected super-admin role).
- Workspaces & projects: list, owners, usage (issues, members, storage), suspend a workspace.
- System: health, version, outbox lag, relay errors, audit log (guard `audit`).
- Every action is audited; super admins cannot demote the last super admin (guard invariant).
Decision pending (see chat): Kyber-native React admin backed by guard `access` + `audit`, versus mounting
guard's server-rendered `adminui` (requires guard sessions, Postgres + Redis).

## S38 — Hosted sign-up ("use Kyber like Jira Cloud")
Anyone can sign up on the site, create a workspace and invite their team; depends on S33 (workspaces).
Email verification (emailx), password reset, invitations by email, per-workspace limits configured by super admins.

## S33 — Workspaces, phase 1 ✅ (shipped; see ADR-0004 implementation note)

## S41 — Pomodoro on issues, with history
As a developer I want to work on an issue in Pomodoro sessions and keep a history of them.
- Start a 25-minute focus session on an issue (configurable 15–60), pause/resume, stop early, 5/15-minute breaks.
- One running session per user; the timer survives reloads (server-side start time, client countdown).
- Every finished or stopped session is stored as a work-log entry (issue, user, start, end, focused minutes,
  completed or interrupted) and appears in the issue's history; "time spent" sums them (Jira worklog parity).
- Reports: focus time per person/day and per sprint; streaks.
