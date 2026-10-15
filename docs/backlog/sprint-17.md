# Sprint 17 — Presence across replicas, SSO groups and several SSO providers

Source: product request (2026-10-15). Built DDD + TDD in this order; S107 waits for the product owner's
approval of its ADR.

- ✅ **S105 Presence across replicas** (top priority) — `GET /api/v1/presence` must know everyone online on
  any replica, not only on the one that answers (docs/ops/calls.md "known limit").
  - AC1: a user connected (MQTT or SSE) to replica A is online when replica B is asked.
  - AC2: a user who disconnects is offline everywhere within one heartbeat (≤ 15 s; immediately on the
    replica that saw the disconnect).
  - AC3: when a replica crashes its users go offline within 45 s; no stale "online" users remain.
  - AC4: workspaces stay isolated; dev mode (memory) behaves as before.
  - Design recorded as an amendment to ADR-0013; contract test on memory and PostgreSQL; acceptance test
    with two servers on one database; `web/e2e/replicas.spec.ts` checks presence; ops guide updated.
  - **Done:** `internal/platform/presence` (Roster, memory and PostgreSQL stores, contract test), migration
    0088, ADR-0013 amendment. Measured: after `kill -9` of a replica its user went offline on the other one
    after 46 s (45 s TTL + polling).
- ✅ **S106 SSO group → role mapping** (ADR-0015 follow-up) — map an IdP group claim (OIDC `groups` or a
  configured claim; the SAML attribute) to workspace admin/member and project roles (ADR-0003).
  - AC1: site admins configure the mapping; AC2: applied at every SSO login (grant and revoke);
    AC3: every change is audited; AC4: users without a mapped group keep today's behaviour.
- **S107 Several SSO providers, per-workspace SSO** (ADR-0015 follow-up) — several OIDC/SAML providers per
  installation; a workspace can require SSO and route users by email domain (ADR-0004).
  - ADR first; **stop for approval** before building.
