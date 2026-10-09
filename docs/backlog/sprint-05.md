# Sprint 05 — "Plan the next two weeks" (backlog ranking & Scrum sprints)

**Owner:** Product Team · **Executes:** Backend, Frontend · **Verifies:** Quality

## Problem
Teams can track issues but cannot *plan*: there is no ordered backlog and no time-boxed sprint, which is
the core reason Scrum teams use Jira. Without it Kyber is a Kanban toy for them.

## Sprint goal (measurable)
A team ranks its backlog by drag-and-drop, plans a sprint, starts it (the board then shows only that
sprint), and completes it — unfinished work returns to the backlog automatically.
**Done =** all ACs automated (3 backends + contract), UI tests, e2e of plan → start → complete.

## Non-goals
Velocity/burndown charts, sprint reports, multiple parallel active sprints, story points, epics roadmap.

## Stories

### KYB-S18 — Backlog ranking  ·  P0
- AC1: every issue has a `rank`; new issues go to the **bottom** of the backlog.
- AC2: `POST /api/v1/issues/{KEY-N}/rank {"after": "KEY-M"}` or `{"before": "KEY-M"}` moves it; ordering is stable and never needs a full re-number (fractional ranks).
- AC3: `GET /api/v1/projects/{key}/issues` is ordered by rank; `?sprint=backlog` / `?sprint={id}` filter.
- AC4: members only (viewer `403`, outsider `404`); the anchor must be in the same project (`422`).

### KYB-S19 — Sprints  ·  P0
- AC1: `POST /api/v1/projects/{key}/sprints {name, goal?}` creates a **planned** sprint; `GET` lists sprints like Jira: active, then planned (oldest first), then closed (newest first).
- AC2: `PATCH /api/v1/issues/{KEY-N} {"sprint_id": id|null}` moves an issue into a planned/active sprint of the same project, or back to the backlog; closed or foreign sprints → `422`.
- AC3: `POST /api/v1/sprints/{id}/start` — only one active sprint per project (`409`); only planned sprints start.
- AC4: `POST /api/v1/sprints/{id}/complete` — only the active sprint; every issue not `done` returns to the backlog; result reports `{completed, returned}` counts.
- AC5: `sprint.created|started|completed` events in the outbox; only members with write access manage sprints.

### KYB-S20 — Backlog view & sprint-aware board (UI)  ·  P0
- AC1: a **Backlog** tab: sprint sections (planned/active) above the backlog list; drag to reorder and to move issues between backlog and sprints.
- AC2: create, start and complete sprints from the backlog view; completing shows how many issues returned.
- AC3: when a sprint is active the **Board** shows only its issues and its name/goal; otherwise all issues.
