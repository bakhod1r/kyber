# Decision records (ADRs)

Architecture decisions live in
[`docs/adr`](https://github.com/bakhod1r/kyber/tree/main/docs/adr). Short summaries:

| ADR | Decision |
|---|---|
| [0002 — Persistence: pgx without sqlc](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0002-persistence-pgx-without-sqlc.md) | Hand-written `pgx/v5` repositories behind domain ports; no ORM, no code generation; a shared repository contract suite runs against memory and PostgreSQL. |
| [0003 — Access control: RBAC + ABAC](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0003-access-control-rbac-abac.md) | An `access` context: permission schemes (grants to roles, users, reporter/assignee), read-only statuses and issue security levels, decided by guard's ABAC engine. |
| [0004 — Workspaces on subdomains](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0004-workspaces-and-subdomains.md) | A workspace is a tenant addressed by a slug; `KYBER_BASE_DOMAIN` switches from single-tenant to `<slug>.<base>` multi-tenant mode; sessions are per workspace. |
| [0005 — Classic four-layer DDD](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0005-classic-ddd-layers.md) | Every context has `domain`, `application`, `infrastructure`, `interfaces`, in the backend and the frontend; enforced by architecture tests. |
| [0006 — OpenTelemetry tracing](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0006-opentelemetry-tracing.md) | Spans for HTTP requests, SQL statements and outbox events, exported over OTLP/HTTP only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set. |
| [0007 — Event envelope and dead letters](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0007-event-envelope-and-dead-letters.md) | Outbox rows carry actor, trace parent and schema version; a message failing 10 times is parked as a dead letter, managed with `kyber outbox`. |
| [0008 — Live updates over SSE](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0008-live-updates-sse.md) | *Superseded by 0013.* Hints, not data, streamed per authorized project. |
| [0009 — Chat and calls](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0009-chat-and-calls.md) | Chat and Call contexts: channels, DMs, threads; WebRTC mesh huddles (up to 6) with signaling relayed by Kyber. |
| [0009 — MCP server](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0009-mcp-server.md) | Streamable HTTP MCP endpoint at `/api/v1/mcp`; tools run as the calling user on the same application services as the REST API. |
| [0010 — PostgreSQL row-level security](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0010-postgres-row-level-security.md) | Forced RLS on every table with `workspace_id`; Kyber must connect as a non-superuser role. |
| [0011 — Attachments](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0011-attachments.md) | Files on disk or S3/MinIO, at most 25 MiB, permissions follow the issue; only safe types are served inline. |
| [0012 — Personal access tokens](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0012-personal-access-tokens.md) | `kpat_` tokens stored as SHA-256 hashes, optional expiry, revocable; a token cannot manage tokens. |
| [0013 — MQTT live updates](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0013-mqtt-live-updates.md) | Embedded MQTT broker; WebSocket for browsers, optional plain MQTT for integrations; project hint topics and personal topics. |
| [0014 — Webhooks and GitHub links](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0014-webhooks.md) | Signed outgoing webhooks with identifiers only, retries and SSRF protection; GitHub push/PR events link to issue keys. |
| [0015 — SSO providers](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0015-sso-providers.md) | Generic OpenID Connect (requires `email_verified`) and GitHub sign-in, linking verified emails to existing accounts. |
| [0016 — OAuth 2.1 for MCP clients](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0016-mcp-oauth.md) | Kyber is its own authorization server: dynamic registration, PKCE, consent in the web app, 90-day tokens. |
| [0017 — SAML and SCIM](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0017-saml-and-scim.md) | SP-initiated SAML 2.0 sign-in and SCIM 2.0 user provisioning; deprovisioning deactivates instead of deleting. |
