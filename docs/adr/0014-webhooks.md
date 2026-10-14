# ADR-0014: Outgoing webhooks and GitHub development links

Status: accepted (2026-10-14)

## Context

Teams wire Kyber into CI, chat and scripts. They need a push of what happened in a project, and
they want to see on an issue which commits and pull requests worked on it. Both cross a trust
boundary: outgoing calls can be aimed at internal services (SSRF), event payloads can contain issues
hidden by security levels, and the GitHub endpoint must accept unauthenticated requests.

## Decision

- **Context.** `internal/webhook` (four layers): `webhooks`, `webhook_deliveries` and
  `development_links` (migration 0052, `workspace_id` + RLS per ADR-0010). Projects and issues are
  referenced by key only.
- **Management.** Project administrators (ADMINISTER_PROJECTS through an ACL to Project/Access)
  create, list, update and delete webhooks: `url` (https; plain http only for `localhost`/loopback),
  `events` (names such as `issue.created`, or `*`) and `active`. A 32-byte secret is generated and
  returned only in the create response; it is stored in clear because every delivery is signed with it.
- **Enqueueing.** A `relay.HandleAll` handler reads the event's project (`project`, or the project
  part of `key`/`issue_key`) and queues one delivery per active matching webhook. The delivery key
  is `<outbox message id>/<webhook id>`, so relay redelivery never duplicates a delivery.
- **Payload.** `{id, event, project, occurred_at, data}` POSTed as JSON with `X-Kyber-Event`,
  `X-Kyber-Delivery` (= `id`) and `X-Kyber-Signature: sha256=<hex HMAC-SHA256(body, secret)>`.
  **`data` carries identifiers only** (`id`, `key`, `issue_id`, `issue_key`, `project`,
  `sprint_id`, `version_id`, `meeting_id`, `board_id` when present). Event payloads are written for
  in-process handlers and include titles, comment bodies and user ids, also of issues a security
  level hides from some members; a webhook is configured by an admin but read by an external system,
  so it gets only "what happened to which issue" and fetches details through the authorized API
  with its own token. The body is fixed at enqueue time, so retries send identical bytes.
- **Delivery.** A dispatcher loop claims due deliveries across workspaces (`FOR UPDATE SKIP LOCKED`,
  2-minute lease, like notification deliveries) and POSTs them. 2xx is delivered; anything else is
  retried with exponential backoff (30 s doubling, capped at 30 min) up to 8 attempts, then
  `failed`. The latest HTTP status and error are recorded; admins see the 20 latest deliveries per
  webhook. Deliveries of an inactive webhook fail without being sent.
- **SSRF.** The sender checks the address actually dialed (after DNS resolution, so rebinding cannot
  bypass it) and refuses loopback, private, link-local, CGNAT, multicast and unspecified addresses
  unless `KYBER_WEBHOOK_ALLOW_PRIVATE=true`; a refusal fails the delivery permanently. Requests have a
  10-second timeout, never follow redirects and never use a proxy.
- **GitHub.** `POST /api/v1/integrations/github` sits outside session authentication; each request
  must carry `X-Hub-Signature-256` for the shared secret `KYBER_GITHUB_WEBHOOK_SECRET` (constant-time
  compare; the endpoint is 404 when the secret is unset). The workspace comes from the host as for
  every request. `push` commits link to the keys in their messages, `pull_request` events to the keys
  in the title and head branch (`[A-Z][A-Z0-9]+-\d+`, at most 20 keys each); links are upserted by
  (issue, kind, repository, ref), so PR state changes update the record. Other events are
  acknowledged and ignored.
- **Reading links.** `GET /api/v1/issues/{key}/development` authorizes browse on the issue (security
  levels included); links are never listed on their own, so links that name hidden or not-yet-existing
  issues reveal nothing.

## Consequences

- One global GitHub secret per installation (per-project secrets can be added later as rows keyed by
  project). Anyone holding it can attach links to any key of the workspace, which is only shown to
  people who may see that issue.
- Receivers must fetch details through the API; webhooks are hints with identifiers, not data exports.
- Webhook secrets are readable to database administrators; rotating one means deleting and re-adding
  the webhook.
