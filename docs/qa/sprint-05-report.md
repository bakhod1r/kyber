# QA Report — Sprint 05

**Teams:** Quality + Security · **Verdict:** ✅ Ready for **v0.5.0**

## Acceptance criteria coverage (Go ACs on memory, PostgreSQL, PostgreSQL + Redis)
| Story | Evidence | Result |
|---|---|---|
| S18 Backlog ranking | `TestS18BacklogRanking` ×3; rank property tests (3 000 random inserts, 10 000 appends/prepends ≤ 6 chars, 200 dense inserts); app `TestRankMoves` (top, bottom, neighbour swaps, self-anchor); repo contract (order, neighbours, sprint filter) | PASS |
| S19 Sprints | `TestS19Sprints` ×3 (create, plan, one active, complete returns unfinished, closed sprint takes no issues, next sprint starts); Sprint aggregate 100%; sprint repo contract ×2 | PASS |
| S20 Backlog UI | 8 Vitest tests (sections, cross-section drop = move + rank, same-section = rank only, drop on section, create/start, complete summary, sprint-aware board, quick-add joins sprint); e2e: rank → plan → start → board shows sprint → complete → issue back in backlog | PASS |
| Contract | every new endpoint/field in `api/openapi.yaml` 0.5.0, validated on every response; drift check green | PASS |

## Gates
| Gate | Measured |
|---|---|
| Go `-race`, all packages incl. Postgres/Redis | 100% pass |
| Go coverage | ≥ 85% gate passes |
| Web typecheck / Vitest | clean / 29 of 29 |
| E2E (Chromium) | pass |
| `ctxsentinel`, `npm audit` | 0 / 0 |

## Found & fixed this sprint
| ID | Severity | Finding | Fix |
|---|---|---|---|
| QA-05-1 | **High** | 10 concurrent `start` calls on the *same* sprint: 5 returned 200, each writing a `sprint.started` event (lost update; the DB index only stops *different* active sprints) | Sprint optimistic locking (`version`, migration 0006, `SPRINT_CONFLICT` 409); contract test; re-run: 1 × 200, 9 × 409, exactly 1 event |
| QA-05-2 | High | `GET/POST /sprints/not-a-uuid/...` would reach a Postgres `uuid` cast → 500 | `platform/id.Valid` guard → 404; contract case |
| QA-05-3 | Medium (Jira parity) | Quick-add on the board during an active sprint put the issue in the backlog, so it vanished from the board | new issue joins the active sprint; regression test |

## Exploratory (real binary, PostgreSQL)
| Scenario | Result |
|---|---|
| Upgrade the long-lived QA DB (201 issues from v0.2, migrations 3 → 6) | ranks backfilled `c001…c201`, rank order = number order, new issue `KYB-202` → `c202` at the bottom ✅ |
| 10 concurrent starts of two different sprints | exactly one active sprint in the DB ✅ |
| 60 concurrent random rank moves | list sorted, 0 duplicate ranks; 1 × 409 (client retries) ✅ |
| Server error log during all runs | 0 errors ✅ |

## Known limitations
1. Concurrent rank moves that compute the same slot get `409` and must retry (by design).
2. Completing a sprint returns issues one by one; a crash mid-way leaves the sprint active and completion can simply be re-run.
3. No velocity/burndown yet (non-goal).
