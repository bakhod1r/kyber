// insights: HTTP gateway to the insights API (infrastructure).
import { request, seg } from "../../../shared/http";
import type { Burndown, ControlChart, DayStat, FlowDay, ReportSummary, SprintReport, Velocity } from "../domain/model";

export const insightsGateway = {
  reportSummary: (key: string) => request<ReportSummary>("GET", `/api/v1/projects/${seg(key)}/reports/summary`),
  reportCreatedVsResolved: (key: string, days: number) =>
    request<{ days: DayStat[] }>("GET", `/api/v1/projects/${seg(key)}/reports/created-vs-resolved?days=${days}`),
  reportVelocity: (key: string) => request<Velocity>("GET", `/api/v1/projects/${seg(key)}/reports/velocity`),
  reportControlChart: (key: string, days: number) => request<ControlChart>("GET", `/api/v1/projects/${seg(key)}/reports/control-chart?days=${days}`),
  reportCumulativeFlow: (key: string, days: number) =>
    request<{ days: FlowDay[] }>("GET", `/api/v1/projects/${seg(key)}/reports/cumulative-flow?days=${days}`),
  burndown: (sprintId: string) => request<Burndown>("GET", `/api/v1/sprints/${seg(sprintId)}/burndown`),
  sprintReport: (id: string) => request<SprintReport>("GET", `/api/v1/sprints/${seg(id)}/report`),
};
