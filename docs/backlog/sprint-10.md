# Sprint 10 — Scrum & Kanban at Jira parity

Source: architecture review (2026-10-09). Priority order; each story DDD + TDD with acceptance tests on all backends.

## P0
- **S43 Configurable workflows per project** — statuses with categories (To Do / In Progress / Done), transitions,
  workflow editor; replaces the hard-coded `DefaultWorkflow`.
- **S44 Board aggregate (agile)** — Scrum and Kanban boards: columns mapped to one or more statuses, WIP limits
  (min/max, column turns red), board filter (saved query); several boards per project.
- **S45 Issue hierarchy and links** — parent link (epic → story/task → sub-task) with epic progress; issue links
  blocks / is blocked by / relates to / duplicates / clones.
- **S46 Wire access control (ADR-0003)** — permission schemes and issue security on every endpoint and read model;
  authorize after loading the issue (attribute rules), endpoint × role × attribute matrix tests.

## P1
- **S47 Sprint scope & sprint report** — issues added/removed after start, completed / not completed / removed
  lists, carry-over on completion; estimation statistic (points, issue count, original time estimate).
- **S48 Kanban backlog & releases** — optional backlog column for Kanban boards; fix versions (releases) with
  release notes; resolution field.
- **S49 Event envelope** — workspace_id, project_key, actor, occurred_at, schema version on every event;
  relay retries with dead-letter queue.

## P2
- Swimlanes (assignee, epic, query), quick filters, parallel sprints, CFD and control chart, labels and components.
