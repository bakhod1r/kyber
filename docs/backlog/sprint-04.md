# Sprint 04 — "Work on an issue together" (issue details, comments, API contract)

**Owner:** Product Team · **Executes:** Backend, Frontend · **Verifies:** Quality

## Problem
v0.3 issues are only a title and a status. Teams cannot say *who* does the work, *how urgent* it is,
*what* exactly is needed, or discuss it — so they keep using another tool next to Kyber.

## Sprint goal (measurable)
From the board, a member opens an issue, edits title/description/priority, assigns a project member,
and comments; two people editing the same issue never silently overwrite each other.
**Done =** all ACs automated on both backends + UI tests + e2e; every API response validated against `api/openapi.yaml`.

## Non-goals
Rich-text/Markdown rendering (plain text with line breaks only), comment edit/delete, mentions,
notifications, attachments, custom fields, backlog/sprints (Sprint 05).

## Stories

### KYB-S15 — Issue details  ·  P0
- AC1: issues have `description` (≤ 20 000 chars), `priority` (`lowest|low|medium|high|highest`, default `medium`), optional `assignee_id`, and `version`.
- AC2: `PATCH /api/v1/issues/{KEY-N}` with `version` and any of `title`, `description`, `priority`, `assignee_id` (`null` = unassign) → `200` with the updated issue.
- AC3: stale `version` → `409`; invalid priority / blank title / too-long description → `422`.
- AC4: the assignee must be a project member → otherwise `422`; viewers cannot edit (`403`); outsiders `404`.
- AC5: `issue.edited` (changed fields) and `issue.assigned` (`from`, `to`) events go to the outbox.

### KYB-S16 — Comments  ·  P0
- AC1: `POST /api/v1/issues/{KEY-N}/comments {body}` → `201` with `{id, author_id, author_name, body, created_at}`; body 1–10 000 chars after trim, else `422`.
- AC2: `GET /api/v1/issues/{KEY-N}/comments` → oldest first.
- AC3: viewers may read but not comment (`403`); outsiders `404`.
- AC4: `comment.added` event in the outbox.

### KYB-S17 — Issue panel in the UI  ·  P0
- AC1: clicking a card opens an issue panel (dialog) with editable title, description, priority, assignee (project members).
- AC2: saving sends the loaded `version`; a `409` shows "changed by someone else" and reloads the issue.
- AC3: comments list + add comment.
- AC4: cards show priority and assignee initials.

### KYB-T3 — API contract  ·  P1
- `api/openapi.yaml` (OpenAPI 3) describes every v1 endpoint.
- Acceptance tests validate every response body and status against the spec (contract test), so the spec cannot drift.

### Decision recorded
- ADR-0002: persistence stays hand-written `pgx` behind DDD repository ports — no `sqlc`, no ORM.
