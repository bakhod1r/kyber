# Reports

The **Reports** page of a project shows KPIs, distributions (by status, type, priority, workload) and the
charts below. Each chart is also available from the API.

| Report | Endpoint | What it shows |
|---|---|---|
| Summary | `GET /api/v1/projects/{key}/reports/summary` | KPIs and distributions |
| Burndown | `GET /api/v1/sprints/{id}/burndown` | Remaining story points in a sprint (step line) plus the ideal line |
| Velocity | `GET /api/v1/projects/{key}/reports/velocity` | Committed vs completed story points per sprint |
| Created vs resolved | `GET /api/v1/projects/{key}/reports/created-vs-resolved?days=` | Daily flow of new and resolved issues |
| Cumulative flow (CFD) | `GET /api/v1/projects/{key}/reports/cumulative-flow?days=` | Issues per status category (To Do / In Progress / Done) at the end of each day, oldest first |
| Control chart | `GET /api/v1/projects/{key}/reports/control-chart?days=` | Cycle time (first In Progress → Done) of each issue completed in the period, a rolling average over five issues, the mean and the standard deviation (hours) |

`days` is 1–365 and defaults to 30 for the cumulative flow and control chart.
