# ADR-0007: Event envelope and dead letters

Status: accepted (2026-10-13)

## Context

Side effects across aggregates and contexts (history, auto-watch, notifications, worklog and
version projections, activity) run from domain events through the transactional outbox. The relay
delivers in order and retries a failing message until it succeeds. So one poison message (a handler
bug, a payload it cannot read) stopped every later event of every workspace, with only a warning in
the log. A stored event also did not say who caused it or which request, so its handling could not
be traced back.

## Decision

- **Envelope.** Every outbox row carries, besides name, payload, time and workspace: `actor_id`
  (the requesting user, "" for the system), `trace_parent` (the W3C traceparent of the recording
  request) and `schema_version` (`outbox.SchemaVersion`, now 1). `outbox.MetaOf(ctx, at)` builds
  it for both the PostgreSQL writer (`db.WriteOutbox`) and the in-memory repositories.
- **Trace continuation.** The relay extracts `trace_parent` and opens its `outbox <event>` span as
  a child, so a request and its asynchronous effects form one trace (ADR-0006).
- **Dead letters.** Each failed delivery increments `attempts` and stores `last_error`. When a
  message has failed `MaxAttempts` times (default 10) the relay returns `outbox.DeadLetter`; the
  store parks the row (`dead_at`) and moves on to the next message. Until then a failure still stops
  the batch, so order is kept for transient errors.
- **Operations.** Dead letters stay in the `outbox` table (no second table to keep in sync).
  `kyber outbox dead-letters` lists them and `kyber outbox retry <id>` returns one to the queue
  with its attempts reset, after the handler is fixed. A web view follows with the admin panel (C8).

## Consequences

- A poison message delays its successors by at most `MaxAttempts` relay passes instead of forever.
- A parked message is skipped, so later events of the same aggregate may be handled before it is
  retried. Handlers are already idempotent and key on message ids; a retried event must not
  assume it is the latest.
- Handlers can branch on `Message.Version` when an event's shape changes.
