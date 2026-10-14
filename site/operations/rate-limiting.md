# Rate limiting

| Variable | Default | Limit |
|---|---|---|
| `KYBER_API_RATE_LIMIT` | `1200` | API requests per signed-in user per minute |
| `KYBER_UPLOAD_RATE_LIMIT` | `60` | Attachment and file uploads per user per minute |

Set either to `0` to turn it off. Over the limit the API answers `429` with code `RATE_LIMITED` and a
`Retry-After` header.

Without Redis each replica counts on its own. Set `KYBER_REDIS_URL` to share the budget — and the sign-in
limiter — across all replicas.

Known gap: anonymous OAuth client registration (`POST /api/v1/oauth/register`) is not covered by the per-user
limit; protect it at your reverse proxy if needed.
