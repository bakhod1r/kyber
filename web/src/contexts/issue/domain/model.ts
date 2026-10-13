// issue: domain model (types and pure rules) — no I/O.
/** A status id of the project's workflow (default: todo, in_progress, done). */
export type Status = string;
export type StatusCategory = "todo" | "in_progress" | "done";
export type WorkflowStatus = { id: Status; name: string; category: StatusCategory };
export type Workflow = { initial: Status; statuses: WorkflowStatus[]; transitions: { from: Status; to: Status }[] };
export type IssueType = "epic" | "story" | "task" | "bug" | "subtask";
export type Priority = "lowest" | "low" | "medium" | "high" | "highest";
export type Issue = {
  id: string;
  key: string;
  title: string;
  type: IssueType;
  status: Status;
  description: string;
  priority: Priority;
  assignee_id: string | null;
  reporter_id: string | null;
  estimate: number | null;
  sprint_id: string | null;
  rank: string;
  version: number;
  /** The epic of a story/task/bug, or the issue a sub-task belongs to. */
  parent_key?: string | null;
  /** "confidential": only admins, the reporter and the assignee see it. */
  security_level?: "" | "confidential";
  /** How a resolved issue ended; "" while it is not in a Done status. */
  resolution?: "" | Resolution;
  fix_versions?: string[];
  time_tracking?: TimeTracking;
  custom_fields?: Record<string, string>;
  /** Planned start and due dates (YYYY-MM-DD) for the timeline. */
  start_date?: string | null;
  due_date?: string | null;
};
export type Resolution = "done" | "wont_do" | "duplicate" | "cannot_reproduce";
export const RESOLUTIONS: { id: Resolution; label: string }[] = [
  { id: "done", label: "Done" },
  { id: "wont_do", label: "Won't do" },
  { id: "duplicate", label: "Duplicate" },
  { id: "cannot_reproduce", label: "Cannot reproduce" },
];
export type IssuePatch = {
  version: number;
  title?: string;
  description?: string;
  priority?: Priority;
  assignee_id?: string | null;
  sprint_id?: string | null;
  estimate?: number | null;
  security_level?: "" | "confidential";
  resolution?: Resolution;
  fix_versions?: string[];
  original_estimate?: string;
  remaining_estimate?: string;
  custom_fields?: Record<string, string>;
  /** YYYY-MM-DD; "" clears. */
  start_date?: string;
  due_date?: string;
};
export type Comment = { id: string; author_id: string; author_name: string; body: string; created_at: string };
export const STATUSES: { id: Status; label: string }[] = [
  { id: "todo", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

export const STATUS_CATEGORIES: { id: StatusCategory; label: string }[] = [
  { id: "todo", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

export type LinkType = "blocks" | "relates" | "duplicates" | "clones";
export type IssueLink = { id: string; type: LinkType; phrase: string; issue: Issue };
export type HistoryEntry = {
  at: string;
  by_id: string | null;
  by_name: string;
  field: string;
  from: string;
  to: string;
  from_name: string;
  to_name: string;
};
/** Jira's simplified workflow, used until the project's own workflow has loaded. */
export const DEFAULT_WORKFLOW: Workflow = {
  initial: "todo",
  statuses: STATUSES.map((s) => ({ id: s.id, name: s.label, category: s.id as StatusCategory })),
  transitions: [],
};

export const ISSUE_TYPES: IssueType[] = ["task", "story", "bug", "epic", "subtask"];
export const PRIORITIES: Priority[] = ["highest", "high", "medium", "low", "lowest"];

/** Time tracking in seconds and as Jira-style text ("1d 4h"; 1d = 8h, 1w = 5d). */
export type TimeTracking = {
  original_seconds: number;
  remaining_seconds: number;
  spent_seconds: number;
  original: string;
  remaining: string;
  spent: string;
};
export type Worklog = {
  id: string;
  issue_key: string;
  author_id: string;
  author_name: string;
  started: string;
  seconds: number;
  spent: string;
  comment: string;
};

export type CustomFieldType = "text" | "number" | "date" | "select";
export type CustomField = { id: string; project_key: string; name: string; type: CustomFieldType; options: string[]; version: number };
export type CustomFieldInput = { name: string; type?: CustomFieldType; options: string[] };
export const CUSTOM_FIELD_TYPES: { id: CustomFieldType; label: string }[] = [
  { id: "text", label: "Text" },
  { id: "number", label: "Number" },
  { id: "date", label: "Date" },
  { id: "select", label: "Select list" },
];
/** Splits an options text box ("Gold, Silver" or one per line) into options. */
export function parseOptions(text: string): string[] {
  return text
    .split(/[,\n]/)
    .map((o) => o.trim())
    .filter(Boolean);
}

/** One line of the timeline: a bar from offset (days after the plan's start) lasting span days;
 * null for an unscheduled child shown under its epic. */
export type TimelineRow = { issue: Issue; depth: 0 | 1; offset: number | null; span: number | null };
export type TimelinePlan = { from: string; days: number; rows: TimelineRow[]; unscheduled: Issue[] };

const DAY = 86_400_000;
const dayNumber = (d: string) => Date.parse(`${d}T00:00:00Z`) / DAY;

/** The issue's bar as [first, last] day; an issue with one date lasts that day. */
function barOf(is: Issue): [number, number] | null {
  const start = is.start_date ?? is.due_date;
  const due = is.due_date ?? is.start_date;
  return start && due ? [dayNumber(start), dayNumber(due)] : null;
}

/** planTimeline lays out a project's issues (Jira's Timeline): epics with their children below,
 * then other scheduled issues; issues without dates and outside any epic are listed apart. */
export function planTimeline(issues: Issue[]): TimelinePlan {
  const byRank = [...issues].sort((a, b) => (a.rank < b.rank ? -1 : a.rank > b.rank ? 1 : a.key.localeCompare(b.key, undefined, { numeric: true })));
  const bars = byRank.map(barOf).filter((b): b is [number, number] => b !== null);
  if (bars.length === 0) return { from: "", days: 0, rows: [], unscheduled: byRank };
  const first = Math.min(...bars.map((b) => b[0]));
  const last = Math.max(...bars.map((b) => b[1]));
  const row = (issue: Issue, depth: 0 | 1): TimelineRow => {
    const bar = barOf(issue);
    return bar ? { issue, depth, offset: bar[0] - first, span: bar[1] - bar[0] + 1 } : { issue, depth, offset: null, span: null };
  };
  const epics = byRank.filter((i) => i.type === "epic");
  const epicKeys = new Set(epics.map((e) => e.key));
  const children = (key: string) => byRank.filter((i) => i.parent_key === key && i.type !== "subtask");
  const sortByBar = (list: Issue[]) => [...list].sort((a, b) => (barOf(a)?.[0] ?? Infinity) - (barOf(b)?.[0] ?? Infinity));
  const rows: TimelineRow[] = [];
  for (const epic of epics) {
    const kids = children(epic.key);
    if (!barOf(epic) && !kids.some(barOf)) continue;
    rows.push(row(epic, 0), ...sortByBar(kids).map((k) => row(k, 1)));
  }
  const loose = byRank.filter((i) => i.type !== "epic" && !(i.parent_key && epicKeys.has(i.parent_key)) && i.type !== "subtask");
  rows.push(...sortByBar(loose.filter((i) => barOf(i))).map((i) => row(i, 0)));
  const shown = new Set(rows.map((r) => r.issue.key));
  const unscheduled = byRank.filter((i) => !shown.has(i.key) && i.type !== "subtask");
  return { from: new Date(first * DAY).toISOString().slice(0, 10), days: last - first + 1, rows, unscheduled };
}
