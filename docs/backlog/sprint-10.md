# Sprint 10 — Scrum & Kanban at Jira parity

Source: architecture review (2026-10-09). Priority order; each story DDD + TDD with acceptance tests on all backends.

## P0
- **S43 Configurable workflows per project** — statuses with categories (To Do / In Progress / Done), transitions,
  workflow editor; replaces the hard-coded `DefaultWorkflow`. ✅ **Done:** migration 0018, `GET/PUT
  /projects/{key}/workflow`, category-aware reports/sprints/import/export, board columns from the workflow,
  editor (rename/add/remove, initial status). Editing explicit transitions in the UI follows with S44.
- **S44 Board aggregate (agile)** — Scrum and Kanban boards: columns mapped to one or more statuses, WIP limits
  (min/max, column turns red), board filter (saved query); several boards per project. ✅ **Done** except the saved
  board filter and a board settings editor in the UI (boards are configured through the API for now).
- **S45 Issue hierarchy and links** — parent link (epic → story/task → sub-task) with epic progress; issue links
  blocks / is blocked by / relates to / duplicates / clones. ✅ **Done:** migrations 0023/0024, children with progress,
  links read from both ends (hidden when the other issue is not readable), both in the issue history and audit log.
- **S46 Wire access control (ADR-0003)** — permission schemes and issue security on every endpoint and read model;
  authorize after loading the issue (attribute rules), endpoint × role × attribute matrix tests. 🟡 **Phase 1 done**
  for Issue Tracking (see ADR-0003 note); phase 2: stored schemes, security levels, the other contexts.

## P1
- **S47 Sprint scope & sprint report** — issues added/removed after start, completed / not completed / removed
  lists, carry-over on completion; estimation statistic (points, issue count, original time estimate).
- **S48 Kanban backlog & releases** — optional backlog column for Kanban boards; fix versions (releases) with
  release notes; resolution field.
- **S49 Event envelope** — workspace_id, project_key, actor, occurred_at, schema version on every event;
  relay retries with dead-letter queue.

## P2
- Swimlanes (assignee, epic, query), quick filters, parallel sprints, CFD and control chart, labels and components.
