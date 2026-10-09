# ADR-0003 — Access control: permission schemes (RBAC) + attribute rules (ABAC)

- **Status:** Accepted (2026-10-09)
- **Replaces:** the fixed viewer < member < admin check in `project/domain.Project.Authorize`

## Context
Jira parity needs more than three ordered roles: fine-grained permissions shared across projects
(permission schemes), "own" permissions (edit/delete only what I reported), issue-level security
(confidential issues) and workflow-dependent rules (closed issues are read-only). Today every context
asks the Project context for `PermRead/PermWrite/PermAdmin`; that is too coarse and cannot see issue
attributes.

## Decision
1. **New bounded context `access`** with a pure `domain/` (no I/O): one function decides
   `Decide(Subject, Permission, Resource) → Decision{Allowed, Reason}`.
   **The decision engine is guard's ABAC** (`github.com/bakhod1r/guard/access/domain.Decide`, already a
   dependency): a Kyber scheme is compiled into guard policies — grants become `allow` policies whose
   conditions test `subject.project_roles`, `subject.id` or `$resource.reporter/assignee`; read-only
   statuses and security levels become `deny` policies that win by priority. Kyber keeps only its
   ubiquitous language (schemes, holders, levels). Compiled policies pass guard's own `Validate`, so they
   can later be stored in guard's Postgres tables and edited in guard's admin UI.
2. **RBAC — permission schemes.** A scheme maps each `Permission` to a list of `Grant`s. A grant's holder is
   a *project role* (admin, member, viewer or custom), a *specific user*, or an *attribute holder*
   (`reporter`, `assignee`). Schemes are shared by projects; a default scheme reproduces today's behaviour.
3. **ABAC — attributes.**
   - Holders `reporter`/`assignee` are evaluated against the issue's attributes.
   - `*_OWN_*` permissions (edit/delete own issue, edit/delete own comment) match the author/reporter.
   - **Issue security levels**: an issue with a level is visible only to subjects matching one of the
     level's grants *and* holding `BROWSE_PROJECTS`.
   - **Read-only statuses**: the scheme lists statuses in which editing permissions are denied.
4. **Default deny.** No matching grant → denied. Every decision carries a reason (for audit and 403 detail).
   Unknown permissions and an empty subject are denied.
5. **No implicit super-user.** Workspace owners administer projects but do not bypass issue security;
   visibility of a confidential issue is always explicit.
6. **Enforcement point.** Application services call an `Authorizer` port before every use case; HTTP
   handlers never decide. Lists (issues, search, reports, export, MCP, SSE) are **filtered by the same
   decision**, so a hidden issue cannot leak through aggregates.
7. **Errors.** A subject that may not `BROWSE_PROJECTS` gets 404 (no existence leak); one that can browse
   but lacks the permission gets 403; an issue hidden by a security level is 404.

## Threat model (STRIDE, condensed)
| Threat | Mitigation | Test |
|---|---|---|
| Elevation: member edits project settings | scheme check `ADMINISTER_PROJECTS` | endpoint × role matrix |
| Information disclosure: confidential issue in lists/reports/export/search | list filtering through `Decide` | leak tests per read model |
| Information disclosure: existence probing via 403 vs 404 | non-browsers always 404 | matrix asserts status codes |
| Tampering: editing a closed issue | read-only statuses | domain + acceptance tests |
| Spoofing "own": changing reporter to gain edit rights | reporter is immutable through the API | domain test |
| Repudiation | denials are logged with reason | audit test |
| Cross-tenant access (with workspaces, ADR-0004) | workspace is part of every subject/resource | tenant isolation suite |

## Testing policy
- `access/domain` is a pure decision table: **100 % statement coverage is required** (CI gate per package).
- Every HTTP endpoint is covered by an authorization matrix (roles × attributes × expected status) on all backends.
- New packages from this ADR on must reach 100 % statement coverage; the global gate (`COVER_MIN`) rises
  as older packages are back-filled. Lines that are unreachable by construction (e.g. panics on static
  definitions) are removed rather than excluded.

## Consequences
- ✅ One place to reason about "who may do what", testable without a database.
- ✅ Jira-like flexibility (schemes, own-permissions, security levels) without a generic policy language.
- ⚠️ Every list endpoint must filter through the decision; the leak tests are mandatory.
- ⚠️ Scheme changes affect many projects at once — edits require `ADMINISTER` and are audited.
