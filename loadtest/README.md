# Load test

`api.js` (k6) seeds a project with `ISSUES` issues, then reads at a constant 50 requests/s for
`DURATION`: one issue, a page of a filtered issue list (`status=todo&limit=50`) and a JQL search
(`project = … AND type = bug ORDER BY key DESC`). Target (PLAN §4 M2): **p95 < 100 ms**, < 1 % errors.

```sh
KYBER_API_RATE_LIMIT=0 KYBER_DATABASE_URL=… ./bin/kyber &      # one user sends 50 req/s: lift the per-user limit
KYBER_URL=http://localhost:8080 ISSUES=10000 DURATION=2m "$(scripts/dev-env.sh k6)" run loadtest/api.js
```

## Results

2026-10-14, one replica, 4 vCPU container, PostgreSQL 16 on the same host, 10 000 issues per project
(20–30 000 in the database), k6 v0.57.0.

| Run | Change | Reads p50 | p95 | p99 | Errors |
|---|---|---|---|---|---|
| 1 | baseline (list without paging, 5.4 MB per response) | 6.2 s | 11.8 s | 12.3 s | 0 % (requests dropped) |
| 2 | `limit`/`offset` paging on `GET /projects/{key}/issues` | 489 ms | 1.32 s | 1.75 s | 0 % |
| 3 | JQL evaluator: build the value normaliser once (was 56 % of search CPU) | 5.3 ms | **78.9 ms** | 104.7 ms | 0 % |
| 4 | v0.1.0 release check (2026-10-16, main `fada1e7` + docs; same setup) | 2.7 ms | **37.4 ms** | — (max 88 ms) | 0 % |

Single requests on an idle server after run 3: issue 3 ms, list page 3 ms, search ~35 ms (a search still
loads every issue of the query's narrowed set; see the JQL pushdown in ADR/PLAN).

Notes: the server connected as the PostgreSQL superuser, so row-level security (ADR-0010) was not in
the path; expect slightly higher numbers through the `kyber` role.
