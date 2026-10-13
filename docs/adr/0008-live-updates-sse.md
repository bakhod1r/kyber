# ADR-0008: Live updates over Server-Sent Events

Status: accepted (2026-10-13)

## Context

Boards, lists and issue panels showed what was loaded; a teammate's change appeared only after a
reload. Domain events already flow through the outbox relay (ADR-0007).

## Decision

- **Hints, not data.** `GET /api/v1/events` is an SSE stream of `change` events carrying only
  `{"project", "event"}`. Titles, keys and other content never travel on the stream, so confidential
  issues (security levels) cannot leak through it; clients refetch through the authorized API.
- **Authorization per hint.** The stream belongs to the caller's workspace, and each hint is sent
  only if the caller may browse its project (Project context, checked when the hint is sent, so a
  revoked membership stops hints at once).
- **Source.** The relay's `HandleAll` hook turns every outbox event with a project (`project`, or the
  project of an issue `key`) into a hint. Hints are best effort: a failure is logged and never
  retried, so it cannot hold up or dead-letter the event.
- **Replicas.** With PostgreSQL, hints go out through `NOTIFY kyber_live` and every replica's
  listener (`LISTEN`, reconnecting) feeds its own hub, so a stream on any replica sees changes relayed
  on any other. In-memory dev mode publishes to the local hub.
- **Back-pressure.** Each stream buffers 64 hints; a slower client gets `resync` instead of an
  unbounded queue. A comment every 25 s keeps proxies from closing idle streams; the stream clears
  the server's write deadline.
- **Client.** `useLive` (web/src/shared/live.ts) opens one `EventSource` for the signed-in app and
  invalidates active TanStack queries at most every 300 ms. The browser reconnects by itself
  (`retry: 3000`).

## Consequences

- Latency is the relay interval (default 1 s) plus the debounce.
- A hint refetches all active queries of the page; precise per-query invalidation can come later.
- Writers wrapping `http.ResponseWriter` must implement `Unwrap` so streams can flush.
