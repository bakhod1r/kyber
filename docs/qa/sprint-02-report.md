# QA Report — Sprint 02

**Teams:** Quality + Security · **Build:** `main` · **Verdict:** ✅ Ready for **v0.2.0** self-hosting behind HTTPS (limitations below)

## Acceptance criteria coverage (every AC runs on **both** memory and PostgreSQL backends)
| Story | Automated evidence | Result |
|---|---|---|
| S6 Durable storage | `repotest.Run` contract on memory + Postgres; `TestMigrateIsIdempotent`; restart smoke | PASS |
| S7 Concurrency | contract `stale version is rejected atomically`; 40 parallel allocations (Postgres row lock) | PASS |
| S8 Outbox | contract: events written with aggregate, none on failed save; `TestEventJSONContract` | PASS |
| S9 Auth | `TestS9Auth` (signup, cookie flags, `/me`, logout, identical 401s); identity app/domain tests | PASS |
| S10 API protected | `TestS10APIRequiresAuth` (no token, forged token, Bearer, cookie) | PASS |
| T2 Operability | `TestOps` (`/healthz`, `/readyz`, `/metrics`); CI Postgres service | PASS |

## Gates
| Gate | Threshold | Measured |
|---|---|---|
| Tests incl. Postgres integration, `-race` | 100% pass | 100% (13 packages) |
| Total coverage | ≥ 85% | 89.5% |
| Domain coverage | ≥ 95% | identity 100%, project 100%, issue ≥ 95% |
| `gofmt`, `go vet` | clean | clean |

## Exploratory (real binary + PostgreSQL 16)
| Scenario | Result |
|---|---|
| 200 concurrent issue creations | 200 unique keys ✅ |
| Restart (SIGTERM → start) | issues, statuses, sessions persist; numbering continues at `KYB-201` ✅ |
| Outbox after run | 201 `issue.created` + 1 `issue.transitioned`, snake_case payloads ✅ |
| Password at rest | `$argon2id$v=19$m=65536,t=1,p=2…` ✅ |
| Session token at rest | only 32-byte SHA-256 hash; raw token absent ✅ |
| API responses | never include password hash ✅ |
| Database stopped | `/readyz` 503, `/healthz` 200, API 500 (no detail leaked) ✅ |
| Database restarted | auto-recovers, `/readyz` 200 ✅ |

### Found & fixed during QA
- **QA-02-1** Outbox payload used Go field names and lacked the issue key → snake_case contract with `key`, locked by `TestEventJSONContract`.

## Security review notes
- ✅ argon2id (OWASP params), 10–128 char passwords (upper bound prevents hash DoS).
- ✅ Constant-time compare; dummy hash verification on unknown users (timing-safe, no enumeration).
- ✅ Cookie `HttpOnly`, `SameSite=Lax`, `Secure` by default (`KYBER_COOKIE_SECURE=false` only for local HTTP).
- ✅ Parameterised SQL everywhere; DB constraints mirror domain invariants.
- ⚠️ **No login rate limiting / lockout** → Sprint 03 (must ship before public exposure).
- ⚠️ No CSRF token; mitigated by SameSite=Lax + JSON-only bodies. Revisit when the web UI lands.
- ⚠️ Expired sessions are rejected but not purged → cleanup job in Sprint 03.
- ⚠️ No per-project authorization yet: any signed-in user can access every project (Sprint 03 RBAC).

## Run it
```bash
docker compose up --build        # Postgres + Kyber on :8080
```
