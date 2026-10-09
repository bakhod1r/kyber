# QA Report — Sprint 03

**Teams:** Quality + Security · **Build:** `main` · **Verdict:** ✅ Ready for **v0.3.0** (team use behind HTTPS; limitations below)

## Acceptance criteria coverage
| Story | Automated evidence (Go ACs run on memory **and** PostgreSQL) | Result |
|---|---|---|
| S11 Membership & roles | `TestS11Membership` (AC2–AC6); project domain/app tests; Postgres repo test | PASS |
| S12 Login throttling | `TestS12LoginThrottling` (429 + Retry-After); `throttle_test.go` (window, reset, per IP) | PASS |
| S13 Session hygiene & CSRF | `TestS13CSRFGuard`; `TestPurgeExpiredSessions`, `TestRunSessionPurgerStopsOnCancel`, Postgres `TestDeleteExpiredSessions` | PASS |
| S14 Web UI | 15 Vitest component tests (auth, projects, board drag/rollback, quick-add, invite); `web/spa_test.go`; `TestS14UIServed`; **Playwright e2e** full flow | PASS |

## Gates
| Gate | Threshold | Measured |
|---|---|---|
| Go tests incl. Postgres, `-race` | 100% | 100% |
| Go coverage | ≥ 85% | 89.6% |
| Web typecheck (strict, no `any`) | clean | clean |
| Web unit tests | 100% | 15/15 |
| E2E (Chromium) | pass | pass |
| `npm audit` (all deps) | 0 high | **0 vulnerabilities** |
| `govulncheck` | 0 reachable | 0 (after pgx fix, CI run #4) |

## Found & fixed this sprint
| ID | Severity | Finding | Fix |
|---|---|---|---|
| SEC-03-0 | **Critical** | CI `govulncheck`: SQL injection in pgx v5.7.5 (GO-2026-5004) + 2 more reachable vulns | pgx v5.11.0, Go 1.26 (`b4dec05`) |
| QA-03-1 | **High** | Upgrading a v0.2 DB orphaned every existing project (no members → 404 for all users, data unreachable) | migration `0003` backfills member-less projects; regression test; verified on a real v0.2 DB (201 issues restored) |
| QA-03-2 | High | Login after an anonymous redirect bounced back to `/login` (stale cached `me`) — found by e2e | refetch `me` before navigating; regression unit test |
| SEC-03-3 | High | `react-router-dom` 7.6.2 had 13 advisories; dev toolchain (vite/vitest/tinypool) had critical/high advisories | react-router 7.18.4, vite 8, vitest 5; CI now fails on `npm audit --audit-level=high` |

## Exploratory (real binary)
| Scenario | Result |
|---|---|
| v0.2 database upgraded in place (migrations 0002 + 0003) | applied once; legacy project reachable ✅ |
| 6th login after 5 failures (correct password) | `429`, `Retry-After: 900` ✅ |
| UI routes `/`, `/projects/X` | SPA shell `200`; missing asset `404` ✅ |
| UI headers | CSP `default-src 'self'`, `frame-ancestors 'none'`, `nosniff`, `Referrer-Policy` ✅ |
| Drag `todo → done` in browser | rejected, alert shown, card returns ✅ |

## Known limitations (accepted for v0.3)
1. Login throttle is **per process**; multi-replica deployments need a shared store (Redis/Postgres) — Sprint 04.
2. Client IP is the TCP peer; behind a reverse proxy all users share the proxy IP (configure throttling at the proxy, or wait for trusted `X-Forwarded-For` support).
3. No email invitations — invitees must sign up first, then an admin adds them by email.
4. `CLAUDE.md` conventions not yet met: OpenAPI-first contract and `sqlc` (tech debt, Sprint 04).
