# Release gate (`make ship`)

A version is tagged only after `make ship` passes on the commit to be released, with PostgreSQL
available (`KYBER_TEST_DATABASE_URL`) and Chromium for the end-to-end tests (`KYBER_CHROMIUM`).

| Step | Command | Checks |
|---|---|---|
| Format and vet | `make lint` | `gofmt`, `go vet` |
| Web build and tests | `make web web-test` | strict TypeScript, unit tests |
| Go tests | `make test` | `-race`, memory and PostgreSQL adapters, OpenAPI contract, coverage floor |
| Strict coverage | `make cover-strict` | listed packages at 100 % |
| End-to-end | `make e2e` | Playwright against a fresh server |
| Docs site | `make docs` | VitePress build, no dead links |
| Security | `make audit` | `govulncheck`, `gosec` (triaged rules excluded, see below), `npm audit` for web and site |

Then, by hand:

1. Read the diff since the previous tag for migrations; check they are forward-compatible
   ([upgrade.md](upgrade.md)).
2. Run the load test against a PostgreSQL instance ([loadtest/README.md](../../loadtest/README.md))
   and compare with the last recorded numbers.
3. Update the changelog / release notes, then tag: `git tag -s vX.Y.Z && git push origin vX.Y.Z`.
   The `release` workflow builds the binaries (GoReleaser) and the multi-arch image.

## gosec rules excluded after triage (2026-10-14)

- **G101** (hard-coded credentials): matches error-code names and SQL column lists, no secrets.
- **G104** (unchecked errors): only `Close` calls on paths that already return an error.
- **G115** (integer conversions): bounded values (weekday bit masks, hash lengths).
- **G304** (file path from variable): blob paths are built from validated keys (`blob.validKey`,
  attachment ids), never from user-supplied paths.
- **G505** (SHA-1): required by the TURN REST API (coturn `use-auth-secret`) for HMAC credentials.
