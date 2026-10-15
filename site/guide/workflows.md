# Workflows

Every project has its own workflow: a set of statuses, in board-column order, and the transitions allowed
between them.

- Each status has a category — **To Do**, **In Progress** or **Done** — used by boards, reports and
  JQL's `statusCategory`.
- One status is the **initial** status of new issues.
- **Transitions** list the allowed `from → to` moves. A workflow without transitions lets every status reach
  every other one.

## Visual editor

Project admins edit the workflow in the web app's workflow editor: add, rename and remove statuses, pick
their categories, and draw transitions on a diagram or tick them in a transition matrix. A status that still
holds issues cannot be removed.

## API

```bash
curl -H "$H" localhost:8080/api/v1/projects/KYB/workflow          # read
curl -H "$H" -XPUT localhost:8080/api/v1/projects/KYB/workflow -d @workflow.json   # replace (admins)
curl -H "$H" -XPOST localhost:8080/api/v1/issues/KYB-1/transitions -d '{"to":"in_progress"}'
```

`PUT` replaces the whole workflow (`initial`, `statuses`, `transitions`); removing a status that still holds
issues answers `409`. Boards created afterwards map their columns to the workflow's statuses.
