// goal: domain model — a project's goals and how they are going (Jira's Goals).
export type GoalStatus = "on_track" | "at_risk" | "off_track" | "done";
export type Goal = {
  id: string;
  project_key: string;
  owner_id: string;
  name: string;
  target_date: string | null;
  jql: string;
  status: GoalStatus;
  done: number;
  total: number;
  percent: number;
  version: number;
};
export type GoalInput = { name: string; target_date: string; jql: string };

export const GOAL_STATUSES: { id: GoalStatus; label: string }[] = [
  { id: "on_track", label: "On track" },
  { id: "at_risk", label: "At risk" },
  { id: "off_track", label: "Off track" },
  { id: "done", label: "Done" },
];

export const statusLabel = (s: GoalStatus) => GOAL_STATUSES.find((g) => g.id === s)?.label ?? s;

/** Days left until the target date as of today ("YYYY-MM-DD"); negative when overdue, null without one. */
export function daysLeft(goal: Pick<Goal, "target_date">, today: string): number | null {
  if (!goal.target_date) return null;
  return Math.round((Date.parse(`${goal.target_date}T00:00:00Z`) - Date.parse(`${today}T00:00:00Z`)) / 86_400_000);
}

/** A short line on the deadline: "3 days left", "Due today", "2 days overdue". */
export function deadlineText(goal: Pick<Goal, "target_date" | "status">, today: string): string {
  const d = daysLeft(goal, today);
  if (d === null) return "No target date";
  if (goal.status === "done") return `Target ${goal.target_date}`;
  if (d === 0) return "Due today";
  const n = Math.abs(d);
  return d > 0 ? `${n} ${n === 1 ? "day" : "days"} left` : `${n} ${n === 1 ? "day" : "days"} overdue`;
}
