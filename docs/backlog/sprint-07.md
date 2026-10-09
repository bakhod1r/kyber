# Sprint 07 — "See how the team is doing" (estimates, activity, reports)

**Owner:** Product Team · **Executes:** Backend, Frontend, Design · **Verifies:** Quality

## Problem
Kyber tracks work but cannot answer the questions leads ask every week: *Will we finish the sprint?
Are we creating work faster than we resolve it? Who is overloaded? How fast do we deliver?*
In Jira these are the Reports pages; without them Kyber cannot replace it for team leads.

## Sprint goal (measurable)
A project has a **Reports** tab with a KPI row and six charts (status, type, priority, workload,
created vs resolved, sprint burndown, velocity), each computed from real history and correct to the
issue, light and dark theme, accessible (table fallback for every chart).
**Done =** report maths unit-tested on hand-computed fixtures; ACs on 3 backends; UI tests; e2e.

## Non-goals
Custom dashboards/gadgets, exports (CSV/PDF), cumulative flow diagram, control chart, time tracking.

## Stories

### KYB-S25 — Estimates & sprint dates  ·  P0
- AC1: issues have an optional **story points** estimate (0–999, up to one decimal); `PATCH … {"estimate": n|null}`; `issue.estimated` event.
- AC2: starting a sprint sets `ends_at` (body `{"ends_at"}` optional, default start + 14 days, must be after start).

### KYB-S26 — Activity log (read model)  ·  P0
- AC1: an **Insights** context records issue/sprint events (created, status changes, sprint moves, estimate changes, sprint start/complete) with their event time, idempotently.
- AC2: it is fed by the outbox relay; replaying an event never double-counts.

### KYB-S27 — Reports API  ·  P0
- AC1: `GET /projects/{key}/reports/summary` — totals, by status / type / priority / assignee (with points), unassigned, open/done, average **cycle time** (in progress → done, last 30 days).
- AC2: `GET /projects/{key}/reports/created-vs-resolved?days=30` — daily created and resolved counts and cumulative lines.
- AC3: `GET /sprints/{id}/burndown` — remaining points and issues over time from start to `ends_at` (scope changes included), plus the ideal line.
- AC4: `GET /projects/{key}/reports/velocity` — committed vs completed points for the last 7 closed sprints.
- AC5: members only (outsiders 404).

### KYB-S28 — Reports UI  ·  P0
- AC1: **Reports** tab: KPI tiles, distribution charts, workload, created-vs-resolved, burndown (active sprint), velocity.
- AC2: every chart has a text/table alternative; works in light and dark themes; responsive.
- AC3: the issue panel and backlog rows show/edit story points; the start-sprint action asks for the end date.
