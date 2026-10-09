# QA Report — Sprint 06

**Teams:** Quality + Security · **Verdict:** ✅ Ready for **v0.6.0**

## Acceptance criteria coverage (Go ACs on memory, PostgreSQL, PostgreSQL + Redis)
| Story | Evidence | Result |
|---|---|---|
| S21 Reporter | `TestS21Reporter` ×3; domain/app tests; contract round-trip; panel shows "Reporter: …" (Vitest + e2e) | PASS |
| S22 Event relay | `outbox` tests: in-order, once, stop-at-failure + retry, cancel; Postgres store: **two concurrent relays, 50 events, each exactly once** (SKIP LOCKED) | PASS |
| S23 Notifications | recipient rules table tests (self, unassign, mention > comment, reporter = assignee, legacy issue); app tests (non-member never notified, idempotent re-delivery ×3 → 1, poison payload skipped); repo contract ×2; `TestS23Notifications` ×3 | PASS |
| S24 UI | 5 Vitest tests (badge, open → mark read → issue opens, mark all, empty state, reporter + `@email` hint); e2e with **two browser sessions** (assign + mention → Bob's bell → deep link to the issue; actor gets nothing) | PASS |
| Contract | OpenAPI 0.6.0: notifications endpoints + `reporter_id`; drift check green | PASS |

## Exploratory (real binaries, PostgreSQL)
| Scenario | Result |
|---|---|
| Upgrade the long-lived QA DB (migrations 6 → 8) | ✅ 202 legacy issues keep `reporter_id = NULL` (never guessed) |
| 213 pre-0.6 unpublished outbox events relayed on first start | ✅ all published, no errors |
| **Two replicas**, 60 concurrent comments mentioning one user | ✅ exactly 60 notifications / 60 distinct events, 0 unpublished, actor 0 |
| Postgres stopped 3 s while the relay runs | ✅ 4 "will retry" warnings, recovers by itself, new comment delivered exactly once |
| Legacy `issue.assigned` without `by` injected into the outbox | ✅ delivered as "assigned by Someone", nothing stuck |

## Found & fixed this sprint
| ID | Severity | Finding | Fix |
|---|---|---|---|
| QA-06-1 | **High** | A pre-0.6 `issue.assigned` event has no actor; looking up user `""` hit a Postgres `uuid` cast → the relay would retry that event forever and **block every later event** | identity `ByID` guards non-UUIDs (not-found); legacy actor shown as "Someone"; regression tests in both packages; verified by injecting a real legacy event |
| QA-06-2 | Low | errorx caught a duplicate numeric error code (6030) at startup before it shipped | `NOTIFICATION_NOT_FOUND` = 6040 |

## Known limitations
1. Delivery is near-real-time (relay every 1 s, UI polls every 30 s); no WebSocket push yet.
2. Mentions use `@email`; no `@name` autocomplete yet.
3. The list shows the newest 50 notifications (the unread count is exact).
