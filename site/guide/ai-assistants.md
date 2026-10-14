# AI assistants (MCP)

Kyber is a [Model Context Protocol](https://modelcontextprotocol.io) server at `POST /api/v1/mcp`
(Streamable HTTP). Every tool runs with your own permissions:

`list_projects`, `search_issues` (JQL, at most 50 results), `get_issue`, `create_issue`, `update_issue`, `transition_issue`,
`assign_issue`, `add_comment`, `get_comments`, `list_members`, `list_sprints`.

## Option 1: OAuth 2.1 (no token to copy)

Clients that support MCP authorization only need the URL. On the first `401` they discover Kyber's
authorization server (`/.well-known/oauth-protected-resource`, `/.well-known/oauth-authorization-server`),
register themselves and open a consent page where you click **Allow** (authorization code with PKCE `S256`).

The client receives a personal access token named `MCP: <client>` that lives 90 days; it appears under
**Settings → Access tokens**, where you can revoke it. Set `KYBER_PUBLIC_URL` so the metadata carries your
external URL.

## Option 2: personal access token

Create a token in **Settings → Access tokens** (or `POST /api/v1/me/tokens`). Tokens look like `kpat_…`;
the secret is shown once, can expire after 1–365 days or never, and acts with your full permissions.
Keep it out of shared config files:

```sh
claude mcp add --transport http kyber https://kyber.example.com/api/v1/mcp \
  --header "Authorization: Bearer $KYBER_TOKEN"
```

A token cannot create or revoke tokens, so a leaked token cannot mint successors. Revocation takes effect
at once.

Design: [ADR-0009 (MCP)](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0009-mcp-server.md),
[ADR-0012 (tokens)](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0012-personal-access-tokens.md),
[ADR-0016 (OAuth)](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0016-mcp-oauth.md).
