# Install & run

Kyber is one Go binary that serves the React UI and the API. It needs PostgreSQL for persistence;
without a database it runs in an in-memory dev mode that keeps nothing.

## Docker Compose

The repository's `docker-compose.yml` starts PostgreSQL, Redis and Kyber:

```bash
git clone https://github.com/bakhod1r/kyber.git && cd kyber
docker compose up --build    # PostgreSQL + Kyber (UI + API) on http://localhost:8080
```

Kyber connects as the plain `kyber` role (created by `deploy/compose/initdb`), not as the superuser, so
PostgreSQL row-level security applies. Override the passwords with `POSTGRES_ADMIN_PASSWORD` and
`KYBER_DB_PASSWORD`. The compose file sets `KYBER_COOKIE_SECURE=false` for plain HTTP — set it to `true`
behind HTTPS.

### `--profile dev`: MinIO and Mailpit

```bash
docker compose --profile dev up --build
```

This adds:

| Service | What | Ports |
|---|---|---|
| `minio` (+ `minio-init`) | S3-compatible storage; the `kyber` bucket is created automatically | `9000` API, `9001` console |
| `mailpit` | Mail catcher | `8025` web UI; SMTP on `mailpit:1025` inside the network |

To use them, uncomment the `KYBER_S3_*` and `KYBER_SMTP_*` lines in the `kyber` service of
`docker-compose.yml` (`KYBER_S3_ENDPOINT: minio:9000`, `KYBER_SMTP_HOST: mailpit`, …).

### `--profile calls`: coturn for huddles

```bash
docker compose --profile calls up -d
```

Starts a `coturn` STUN/TURN server (host networking) so video huddles work between people on different
networks. Use the same long random secret for coturn's `--static-auth-secret` and `KYBER_TURN_SECRET`, point
`KYBER_ICE_SERVERS` at your host, and open 3478 UDP+TCP and the relay range 49160–49200 UDP. See
[`docs/ops/calls.md`](https://github.com/bakhod1r/kyber/blob/main/docs/ops/calls.md) for details.

## First steps with the API

```bash
curl -XPOST localhost:8080/api/v1/auth/signup -d '{"email":"me@example.com","name":"Me","password":"a long password"}'
TOKEN=$(curl -s -XPOST localhost:8080/api/v1/auth/login -d '{"email":"me@example.com","password":"a long password"}' | jq -r .token)
H="Authorization: Bearer $TOKEN"
curl -H "$H" -XPOST localhost:8080/api/v1/projects -d '{"key":"KYB","name":"Kyber"}'
curl -H "$H" -XPOST localhost:8080/api/v1/projects/KYB/issues -d '{"title":"Login page","type":"task"}'
curl -H "$H" -XPOST localhost:8080/api/v1/issues/KYB-1/transitions -d '{"to":"in_progress"}'
```

The full contract is [`api/openapi.yaml`](https://github.com/bakhod1r/kyber/blob/main/api/openapi.yaml)
(OpenAPI 3.1). Errors are RFC 9457 `application/problem+json` with a stable `code`.

## Binary

Build from source (Go and Node.js required):

```bash
make web      # build the React UI into web/dist (embedded by the Go build)
make build    # bin/kyber
KYBER_DATABASE_URL=postgres://kyber:secret@localhost:5432/kyber ./bin/kyber
```

Migrations run automatically at startup. Leave `KYBER_DATABASE_URL` unset to try Kyber in memory.
A container image is built by the repository's `Dockerfile` (distroless, non-root, port `8080`).

Health endpoints: `/healthz` (liveness), `/readyz` (database), `/metrics` (Prometheus).

Next: the [configuration reference](./configuration).
