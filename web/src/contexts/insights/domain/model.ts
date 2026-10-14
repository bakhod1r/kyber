// insights: domain model (types and pure rules) — no I/O.
import type { SprintState } from "../../agile/domain/model";

export type Bucket = { key: string; count: number; points: number };
export type ReportSummary = {
  total: number; open: number; done: number; unassigned: number; total_points: number; open_points: number;
  by_status: Bucket[]; by_type: Bucket[]; by_priority: Bucket[];
  workload: { user_id: string | null; name: string; count: number; points: number }[];
  cycle_time: { count: number; average_hours: number; median_hours: number };
};
export type DayStat = { date: string; created: number; resolved: number; cum_created: number; cum_resolved: number };
export type BurndownSample = { at: string; remaining_points: number; remaining_issues: number };
export type Burndown = {
  sprint: { id: string; name: string; state: SprintState; started_at: string | null; ends_at: string | null; completed_at: string | null };
  start_points: number; start_issues: number; added_points: number; added_issues: number; removed_issues: number;
  samples: BurndownSample[]; ideal: BurndownSample[];
};
export type Velocity = { sprints: { id: string; name: string; committed_points: number; completed_points: number; completed_issues: number }[] };
export type SprintReportIssue = { key: string; title: string; type: string; points: number; status_category: string; added_after_start: boolean };
export type SprintReport = {
  sprint: { id: string; name: string; state: SprintState; started_at: string | null; ends_at: string | null; completed_at: string | null };
  committed_points: number;
  committed_issues: number;
  added_points: number;
  added_issues: number;
  completed_points: number;
  time_original_seconds?: number;
  time_spent_seconds?: number;
  time_remaining_seconds?: number;
  completed: SprintReportIssue[];
  not_completed: SprintReportIssue[];
  removed: SprintReportIssue[];
};

/** Control chart: each done issue's cycle time with a rolling average (hours). */
export type ControlChart = {
  days: number;
  average_hours: number;
  stddev_hours: number;
  points: { key: string; title: string; done_at: string; cycle_hours: number; rolling_hours: number }[];
};

/** Cumulative flow: issues per status category at the end of each day. */
export type FlowDay = { date: string; todo: number; in_progress: number; done: number };

/** stackFlow turns flow days into the CFD's stacked bands: done, done + in progress, and all. */
export function stackFlow(days: FlowDay[]): { x: Date; done: number; started: number; total: number }[] {
  return days.map((d) => ({ x: new Date(`${d.date}T00:00:00Z`), done: d.done, started: d.done + d.in_progress, total: d.done + d.in_progress + d.todo }));
}
