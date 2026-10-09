# Sprint 01 — "First issue on the board" (API slice)

**Owner:** Product Team (product-manager, product-owner) · **Executes:** Backend Team · **Verifies:** Quality Team

## Problem
A team cannot try Kyber at all yet: there is no way to create a project, file an issue, and move it
through a workflow. Until that loop exists, no other feature has value.

## Sprint goal (measurable)
Through the REST API a user can create a project, create issues with keys `KEY-1, KEY-2, …`,
read them, list them by status, and move them through the default workflow.
**Done =** every acceptance criterion below is covered by an automated test and `make test` is green in CI.

## Non-goals (this sprint)
Authentication, persistence in Postgres (in-memory adapters only), UI, custom workflows, comments.

## Kill criterion
If the domain model cannot express the default workflow without hardcoded `if status == …`, stop
and revisit ADR-0008 before building more.

## Stories

### KYB-S1 — Create project  ·  priority P0
As a team lead I want to create a project with a short key so issues get readable IDs.
- AC1: `POST /api/v1/projects {key,name}` → `201` with `{id,key,name}`.
- AC2: key must match `^[A-Z][A-Z0-9]{1,9}$`, else `422`.
- AC3: empty name → `422`.
- AC4: duplicate key → `409`.
- AC5: `GET /api/v1/projects` lists projects; `GET /api/v1/projects/{key}` → `200` or `404`.

### KYB-S2 — Create issue  ·  P0
As a team member I want to file an issue in a project.
- AC1: `POST /api/v1/projects/{key}/issues {title,type}` → `201`, key `KEY-<n>`, `n` increments per project starting at 1.
- AC2: new issue has status `todo` (default workflow initial status).
- AC3: blank title or unknown type (`epic|story|task|bug|subtask`) → `422`.
- AC4: unknown project → `404`.
- AC5: an `issue.created` domain event is published.

### KYB-S3 — View issue  ·  P0
- AC1: `GET /api/v1/issues/{issueKey}` → `200` with `{id,key,title,type,status}`.
- AC2: malformed key → `400`; unknown → `404`.

### KYB-S4 — Move issue through workflow  ·  P0
Default workflow: `todo ⇄ in_progress ⇄ done`.
- AC1: `POST /api/v1/issues/{issueKey}/transitions {to}` → `200` with updated issue.
- AC2: transition not allowed (e.g. `todo → done`) → `409`, status unchanged.
- AC3: an `issue.transitioned` event is published with `from`/`to`.

### KYB-S5 — List project issues  ·  P1
- AC1: `GET /api/v1/projects/{key}/issues` → issues ordered by number.
- AC2: `?status=in_progress` filters by status.

### KYB-T1 — Engineering baseline  ·  P0
- `make test` (race + coverage), `make lint` (`go vet`, `gofmt`), `make run`.
- GitHub Actions CI on push/PR to `main`.
- `/healthz` endpoint; structured `slog` logging.
