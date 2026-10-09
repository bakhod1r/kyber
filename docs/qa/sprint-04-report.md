# QA Report — Sprint 04

**Teams:** Quality + Security · **Verdict:** ✅ Ready for **v0.4.0**

## Acceptance criteria coverage
Go acceptance tests now run on **three backends**: memory, PostgreSQL, PostgreSQL + Redis (guard limiter).

| Story | Evidence | Result |
|---|---|---|
| S15 Issue details | `TestS15IssueDetails` (edit, unassign via `null`, stale version 409, validation 422, non-member assignee 422, viewer 403, outsider 404); domain `details_test.go`; repo contract "details round-trip" | PASS |
| S16 Comments | `TestS16Comments`; Comment aggregate tests; `RunComments` contract on memory + Postgres | PASS |
| S17 Issue panel | 5 Vitest tests (edit+save with version, 409 reload, comments, Escape/close, card priority/initials/open); e2e extended (edit, assign, comment, persists across reload) | PASS |
| T3 API contract | `contract_test.go` validates **every** acceptance response against `api/openapi.yaml`; drift check fails on undocumented routes and unexercised operations (both proven with deliberate breakage) | PASS |
| S12 (revised) | `TestS12LoginThrottling` ×3 backends; `guardlimit` 100% (two replicas share one Redis; Redis down → error); app fails closed | PASS |
| Problem details | `TestProblemDetails`: `application/problem+json`, stable `code`/`numeric_code`, Uzbek title via `Accept-Language: uz` | PASS |

## Gates
| Gate | Measured |
|---|---|
| Go tests, `-race`, incl. Postgres + Redis | 100% pass |
| Go coverage | 89.6% (domain: issue 98.4%, identity 100%, project 97.9%) |
| Web: strict typecheck, Vitest | clean, 21/21 |
| E2E (Chromium) | pass |
| `ctxsentinel` | 0 findings |
| `npm audit` | 0 vulnerabilities |

## Owner libraries adopted
| Library | Use in Kyber | Notes found while integrating |
|---|---|---|
| `emailx` v0.4.0 | Identity email validation | Closed a real gap (`ali@x..uz` was accepted). Upstream: `Normalize` returns no error for invalid input (`"a b@x.com"`); `IsValid` rejects IDN domains (`münchen.de`); `Normalize` canonicalises Gmail (dots/`+tag`) — Kyber deliberately does not use it for stored addresses. |
| `oneenv` v1.9.2 | Typed config, fail-fast on bad values | DB/Redis passwords redacted in logs. |
| `jitterx` v0.1.2 | Jittered session purge | Avoids synchronized purges across replicas. |
| `errorx` v0.1.0 | Error registry (en/uz/ru), RFC 9457 responses | Root module pulls many driver deps into `go.sum` (not compiled in). |
| `guard` v0.4.0 | `ratelimit` behind the LoginLimiter port | Limiter has no "peek", so S12 changed to counting all attempts (10/15 min). Full guard auth/RBAC not adopted: Kyber's roles are per-project domain rules. |
| `ctxsentinel` v0.1.0 | CI lint | Probe: flags `context.TODO()`, but did **not** flag `context.Background()` inside a function that already has `ctx` — possible upstream gap. |

## Known limitations
1. Comment edit/delete, mentions and notifications are not implemented (non-goals).
2. Without `KYBER_REDIS_URL` the login limiter is per process.
3. Error titles are localized; `detail` (the specific reason) is English.
