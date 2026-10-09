# Sprint 03 — "Safe for a real team" (authorization, hardening, first UI)

**Owner:** Product Team · **Executes:** Backend, Frontend, Design · **Verifies:** Quality, Security

## Problem
Sprint 02 QA blocked public exposure: any user sees every project, passwords can be brute-forced,
and there is no UI, so non-developers cannot evaluate Kyber at all.

## Sprint goal (measurable)
A team of 3 can sign up in the browser, create a project, invite each other, and move issues
across a Kanban board — while outsiders get `404` on their project and brute force is throttled.
**Done =** every AC automated; Go suite + web unit tests + one browser e2e green.

## Non-goals
Custom roles, project-level permission schemes, email invitations, Scrum, issue editing beyond
create/transition, dark mode polish.

## Stories

### KYB-S11 — Project membership & roles  ·  P0
Roles: `admin` (manage members) ⊃ `member` (create/transition issues) ⊃ `viewer` (read only).
- AC1: project creator becomes `admin`.
- AC2: non-members get `404` for the project, its issues and `KEY-N` lookups (existence not leaked); `GET /projects` lists only the caller's projects.
- AC3: `viewer` writing (create issue, transition) → `403`.
- AC4: `POST /api/v1/projects/{key}/members {email, role}` — admin only (`403` otherwise); unknown email → `404`; invalid role → `422`; re-adding updates the role.
- AC5: `GET /api/v1/projects/{key}/members` lists members (any member).
- AC6: the last admin cannot be downgraded (`409`).

### KYB-S12 — Login throttling  ·  P0
- AC1: after 5 failed logins for the same email+IP within 15 min → `429` with `Retry-After`, even with the right password.
- AC2: a successful login resets the counter.

### KYB-S13 — Session hygiene & CSRF  ·  P1
- AC1: expired sessions are purged by a background job (hourly).
- AC2: cookie-authenticated non-GET requests without `Content-Type: application/json` → `403` (CSRF guard). Bearer clients unaffected.

### KYB-S14 — Web UI v0 (React)  ·  P0
- AC1: sign up / log in / log out pages; session via cookie.
- AC2: project list + create project.
- AC3: Kanban board per project with `To Do / In Progress / Done` columns; create issue; drag a card to another column triggers the transition; disallowed moves show an error and the card returns.
- AC4: the UI is served by the same Go binary (`embed`), SPA fallback for client routes.
- AC5: strict TypeScript, no `any`; component tests (Vitest) + one Playwright e2e of the full flow.
