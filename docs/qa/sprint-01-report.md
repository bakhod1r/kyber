# QA Report — Sprint 01

**Team:** Quality Engineering · **Build:** `main` · **Verdict:** ✅ Ready for v0.1.0-alpha (scope below)

## Acceptance criteria coverage
| Story | ACs | Automated test | Result |
|---|---|---|---|
| KYB-S1 Create project | AC1–AC5 | `internal/server/acceptance_test.go: TestS1CreateProject` | PASS |
| KYB-S2 Create issue | AC1–AC4 | `TestS2toS5IssueLifecycle` | PASS |
| KYB-S2 AC5 event | AC5 | `internal/issue/app: TestCreateIssue` | PASS |
| KYB-S3 View issue | AC1–AC2 | `TestS2toS5IssueLifecycle/S3` | PASS |
| KYB-S4 Transitions | AC1–AC3 | `TestS2toS5IssueLifecycle/S4`, `app: TestTransitionIssue` | PASS |
| KYB-S5 List/filter | AC1–AC2 | `TestS2toS5IssueLifecycle/S5` | PASS |
| KYB-T1 Baseline | — | `make lint test build`, CI workflow | PASS |

## Quality gates
| Gate | Threshold | Measured |
|---|---|---|
| Unit + acceptance tests, `-race` | 100% pass | 100% pass |
| Total coverage (`./internal/...`) | ≥ 85% | 94.4% |
| Domain coverage | ≥ 95% | issue 95.5%, project 100% |
| `gofmt`, `go vet` | clean | clean |

## Exploratory / smoke (real binary)
| Check | Expected | Result |
|---|---|---|
| 200 concurrent `POST /issues` | 200 unique sequential keys | 200 unique ✅ |
| Unknown route / wrong method | 404 / 405 | ✅ |
| Empty body / 2 MB body | 400 (body capped at 1 MB) | ✅ |
| Injection-shaped key `KYB-1' OR 1=1` | 400 | ✅ |
| Unicode title | 201 | ✅ |
| Transition to unknown status | 409 | ✅ |
| Handler panic | 500, process survives | ✅ (unit test) |
| SIGTERM | graceful shutdown, exit 0 | ✅ |
| Error logs during run | 0 | 0 ✅ |

## Production readiness (release-team checklist)
- ✅ Timeouts: read-header 5s, read/write 15s, idle 60s; body limit 1 MB.
- ✅ Panic recovery, structured JSON logs (`slog`), `/healthz`.
- ✅ Distroless non-root Docker image; static binary.
- ✅ CI: lint, race tests with coverage gate, build, govulncheck, docker build.
- ⚠️ `govulncheck` not run locally (vuln DB blocked in sandbox) — enforced in CI. Stdlib only, no third-party deps.

## Known limitations (accepted for alpha, scheduled)
1. **No persistence** — in-memory storage; data lost on restart → Sprint 02: Postgres adapter + outbox.
2. **No authentication** — must not be exposed publicly → Sprint 02/03: auth (KYB-F1).
3. Events are logged, not durably published (TODO outbox).
4. No `/readyz` or `/metrics` yet.
