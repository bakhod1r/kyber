# Sprint 06 — "Know what needs you" (reporter, @mentions, notifications)

**Owner:** Product Team · **Executes:** Backend, Frontend · **Verifies:** Quality

## Problem
People miss work: nobody is told when an issue is assigned to them, when someone comments on their
issue, or when they are @mentioned. In Jira this is the main way work reaches people.

## Sprint goal (measurable)
Assigning, commenting and @mentioning produce in-app notifications for the right people within seconds,
exactly once, never to the person who acted, and never to non-members.
**Done =** recipient rules unit-tested, relay delivery idempotency proven, ACs green on 3 backends, UI + e2e.

## Non-goals
E-mail/push delivery, watchers list UI, notification preferences, real-time push (polling is fine).

## Stories

### KYB-S21 — Reporter  ·  P0
- AC1: every new issue records its **reporter** (the creator); `reporter_id` in the API (null for issues created before 0.6).
- AC2: the issue panel shows reporter and assignee names.

### KYB-S22 — Event relay  ·  P0
- AC1: a background relay publishes outbox events to in-process handlers (at-least-once, in order).
- AC2: a handler failure leaves the event unpublished and it is retried; delivery is idempotent (no duplicate notifications).
- AC3: multiple replicas never process the same event concurrently (`FOR UPDATE SKIP LOCKED`).

### KYB-S23 — Notifications  ·  P0
Rules (the actor is never notified; non-members are never notified; one notification per person per event):
- AC1: **assigned** — the new assignee is notified ("Alice assigned you KYB-1: Login page").
- AC2: **commented** — the issue's reporter and assignee are notified.
- AC3: **mentioned** — `@email` of a project member in a comment notifies them (takes precedence over "commented").
- AC4: `GET /api/v1/notifications` (newest first, `unread` count, `?unread=true`), `POST /api/v1/notifications/{id}/read`, `POST /api/v1/notifications/read-all`; users see only their own.

### KYB-S24 — Notifications UI  ·  P0
- AC1: a bell in the header with the unread count (polled every 30 s).
- AC2: the panel lists notifications; clicking one marks it read and opens the issue; "Mark all as read".
- AC3: the comment box hints `@email` mentions.
