# ADR-0009: Model Context Protocol server

Status: accepted (2026-10-13)

## Context

Teams use AI assistants (Claude Code, Claude Desktop and others) next to the tracker. The
assistants speak the Model Context Protocol (MCP); without a server they would scrape the REST API
through ad-hoc prompts.

## Decision

- **Transport.** Streamable HTTP at `POST /api/v1/mcp`, one JSON-RPC 2.0 message per request,
  answered with `application/json` (no server-initiated stream; `GET` is 405). Notifications get
  202. Protocol revision 2025-06-18, with 2025-03-26 accepted at `initialize`.
- **Generic layer.** `internal/platform/mcp` knows JSON-RPC and the tool registry only; it imports
  no context. Tools are registered in the composition root (`internal/server/mcp.go`) on top of
  the same application services as the REST API.
- **Identity and permissions.** The endpoint sits behind the existing authentication and
  workspace middleware, so a client uses a normal Bearer session token and every tool runs as that
  user: security levels, project roles and workspace isolation apply unchanged. Calls are audited
  like any other API request.
- **Tools.** `list_projects`, `search_issues` (JQL, at most 50 results), `get_issue`,
  `create_issue` (optional description), `update_issue`, `transition_issue`, `assign_issue`,
  `add_comment`, `get_comments`, `list_members`, `list_sprints` (2026-10-14). A failing tool returns a result with
  `isError: true` and the domain error's message, so the model can correct itself; protocol
  mistakes are JSON-RPC errors.

## Consequences

- Assistants can work on issues without new permissions or a separate credential type.
- Personal access tokens (ADR-0012) give MCP clients long-lived credentials; clients that
  support MCP authorization use OAuth 2.1 instead (ADR-0016).
- New tools are a few lines in `internal/server/mcp.go`, tested through the full HTTP stack.
