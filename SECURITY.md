# Security policy

## Reporting a vulnerability

Please **do not open a public issue**. Report privately through GitHub's
"Report a vulnerability" (Security → Advisories) on this repository. Include the affected version,
steps to reproduce and the impact you expect. We aim to acknowledge reports within 3 working days
and to ship a fix or mitigation within 30 days for high-severity issues, crediting you if you wish.

## Supported versions

The latest release receives security fixes. Self-hosters should upgrade promptly
(see `docs/operations/backup.md`).

## Hardening notes

- Run Kyber as a non-superuser database role so row-level security applies (ADR-0010).
- Serve it over HTTPS with `KYBER_COOKIE_SECURE=true`; terminate TLS for the optional MQTT port.
- Personal access tokens act with their owner's permissions; give them expiries and revoke unused ones.
