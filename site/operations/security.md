# Security notes

## Reporting a vulnerability

Do not open a public issue. Report privately through GitHub's
[security advisories](https://github.com/bakhod1r/kyber/security/advisories) — see
[`SECURITY.md`](https://github.com/bakhod1r/kyber/blob/main/SECURITY.md).

## Hardening checklist

- **Database role.** Connect as a non-superuser role so PostgreSQL row-level security isolates workspaces
  ([ADR-0010](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0010-postgres-row-level-security.md)).
- **HTTPS.** Serve Kyber over HTTPS with `KYBER_COOKIE_SECURE=true` (the default). Terminate TLS for the
  optional plain MQTT port as well.
- **Secrets** belong in the environment or a secret store (the Helm chart only references `existingSecret`),
  never in committed files.
- **Tokens.** Personal access tokens act with their owner's permissions; give them expiries and revoke unused
  ones. Tokens cannot manage other tokens.
- **Webhooks** refuse private and loopback targets unless `KYBER_WEBHOOK_ALLOW_PRIVATE=true`, and carry
  identifiers only.
- **Attachments** are untrusted input; Kyber serves only images and PDF inline, everything else as a download
  ([ADR-0011](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0011-attachments.md)).
- **Leavers.** With SCIM, deactivation in your identity provider ends sessions and tokens at once.
- **Calls.** Keep coturn's `--denied-peer-ip` lines so the relay cannot reach your private networks.
- **Rate limits** are on by default — see [Rate limiting](./rate-limiting).
