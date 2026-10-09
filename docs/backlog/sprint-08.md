# Sprint 08 — "Switch from Jira in an afternoon" (Jira CSV import, CSV export, @mention autocomplete)

**Owner:** Product Team · **Executes:** Backend, Frontend · **Verifies:** Quality, Security

## Problem
Teams evaluating Kyber already have years of issues in Jira. Without an import they would start empty
(and reports would show no history), so they never switch. They also need to get data *out* again.

## Sprint goal (measurable)
A Jira "Export Excel CSV (all fields)" file imports into a Kyber project with a preview first; keys,
types, statuses, priorities, assignees, story points and created/resolved dates map correctly; re-importing
the same file creates nothing twice; reports show the imported history; and the project exports back to CSV.

## Non-goals
Jira REST/API sync, attachments, comments, sprints/epics links, custom workflows, sub-task hierarchy.

## Stories

### KYB-S29 — Jira CSV import  ·  P0
- AC1: `POST /api/v1/projects/{key}/import/jira?dry_run=true` with `{"csv": "..."}` returns a **preview**: rows, to-create, already-imported, per-row errors, and warnings (unknown type/status/priority, unmatched assignee) — nothing is written.
- AC2: without `dry_run`, valid rows are imported; rows with errors are skipped and reported; the response lists `JIRA-KEY → KYB-N`.
- AC3: mapping — Issue Type (Bug/Story/Task/Epic/Sub-task), Status via *Status Category* when present else the status name, Priority (incl. Blocker/Critical/Major/Minor/Trivial), Assignee by email or display name among project members, Story Points, Description.
- AC4: Created and Resolved dates feed the activity log, so reports show the imported history.
- AC5: idempotent — re-importing the same file skips already-imported Jira keys.
- AC6: admins only; files up to 10 MB; quoted fields, multi-line descriptions, BOM and duplicate columns handled.

### KYB-S30 — CSV export  ·  P1
- AC1: `GET /api/v1/projects/{key}/export.csv` — every issue with Jira-compatible headers (Issue key, Summary, Issue Type, Status, Priority, Assignee, Reporter, Story Points, Created, Description); members only.
- AC2: exported files re-import cleanly (round trip).

### KYB-S31 — @mention autocomplete  ·  P1
- AC1: typing `@` and letters in a comment lists matching project members (name or email); Enter/click inserts `@email `.
- AC2: keyboard accessible (↑/↓/Enter/Escape), listbox semantics.

## S32 — MCP server (Model Context Protocol)

As a developer using an AI assistant, I want Kyber exposed as an MCP server so the assistant can
read and update my issues with my own permissions.

Acceptance criteria:
- `kyber mcp` runs an MCP server over stdio; `/api/v1/mcp` serves streamable HTTP.
- Authenticated by a personal API token; every tool call runs under that user's project permissions.
- Tools: `search_issues`, `get_issue`, `create_issue`, `update_issue`, `transition_issue`,
  `add_comment`, `list_sprints`, `project_summary` (insights).
- Resources: `kyber://projects/{key}`, `kyber://issues/{key}`.
- Tool inputs are validated with JSON Schema; errors map to errorx problem codes.
- Contract test drives the server with an MCP client; acceptance tests run on all 3 backends.
