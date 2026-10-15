# Projects & boards

## Projects

A project groups issues under a key such as `KYB`; issues are numbered `KYB-1`, `KYB-2`, …

```bash
curl -H "$H" -XPOST localhost:8080/api/v1/projects -d '{"key":"KYB","name":"Kyber"}'
curl -H "$H" -XPOST localhost:8080/api/v1/projects/KYB/issues -d '{"title":"Login page","type":"task"}'
```

Members have a role per project: `admin` (manage members) ⊃ `member` (create and move issues) ⊃ `viewer`
(read). Non-members get `404`, so a project's existence is not revealed. Finer permission schemes and
issue-level security are described in [ADR-0003](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0003-access-control-rbac-abac.md).

Issues can be edited (`PATCH /api/v1/issues/{KEY-N}` with the `version` you read, for optimistic locking),
commented on with `@mentions`, given attachments (≤ 25 MiB each) and moved between statuses through
transitions.

## Boards

A project can have several boards (`GET/POST /api/v1/projects/{key}/boards`):

- **Scrum** or **Kanban** boards.
- **Columns are mapped to workflow statuses** — the default board follows the project's
  [workflow](./workflows); drag a card to move the issue to that status.
- **WIP limits** per column.
- **Swimlanes** by assignee or by epic.

## Backlog & sprints

- Rank issues in the backlog (`POST /api/v1/issues/{KEY-N}/rank` with `after` / `before` another issue).
- Create sprints, then start and complete them (`POST /api/v1/sprints/{id}/start` · `/complete`).
  Unfinished issues return to the backlog when a sprint completes.

Sprint and project charts are covered in [Reports](./charts).
