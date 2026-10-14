# Webhooks & GitHub

## Outgoing project webhooks

Project admins manage webhooks at `/api/v1/projects/{key}/webhooks[/{id}]` (or in the project settings):

- `url` — HTTPS (plain HTTP only for `localhost`/loopback),
- `events` — names such as `issue.created`, or `*` for all,
- `active`.

A 32-byte secret is generated and returned **only once**, in the create response.

Each delivery is a JSON `POST`:

```json
{ "id": "…", "event": "issue.created", "project": "KYB", "occurred_at": "…", "data": { "key": "KYB-7" } }
```

with the headers `X-Kyber-Event`, `X-Kyber-Delivery` (= `id`) and
`X-Kyber-Signature: sha256=<hex HMAC-SHA256(body, secret)>`. Verify the signature before trusting a request.

`data` carries **identifiers only** (`id`, `key`, `issue_id`, `issue_key`, `project`, `sprint_id`,
`version_id`, `meeting_id`, `board_id` when present). Fetch details through the API with your own token.

Delivery semantics:

- 2xx counts as delivered; anything else is retried with exponential backoff (30 s doubling, capped at
  30 min) for up to 8 attempts, then marked `failed`. Retries send identical bytes.
- Requests time out after 10 seconds, never follow redirects and never use a proxy.
- Loopback, private, link-local, CGNAT and multicast addresses are refused (checked after DNS resolution)
  unless `KYBER_WEBHOOK_ALLOW_PRIVATE=true`.
- `GET /api/v1/projects/{key}/webhooks/{id}/deliveries` shows the 20 latest deliveries.

## GitHub development links

Set `KYBER_GITHUB_WEBHOOK_SECRET` and add a webhook in your GitHub repository:

- Payload URL: `<KYBER_PUBLIC_URL>/api/v1/integrations/github`
- Content type: `application/json`
- Secret: the same value
- Events: **Pushes** and **Pull requests**

Commits that mention an issue key (such as `KYB-123`) in their message, and pull requests that mention it in the
title or head branch, appear under **Development** on the issue (`GET /api/v1/issues/{KEY-N}/development`).
Requests are verified with `X-Hub-Signature-256`; without the secret the endpoint answers `404`.

Design: [ADR-0014](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0014-webhooks.md).
