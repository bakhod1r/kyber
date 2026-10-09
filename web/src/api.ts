// Typed client for the Kyber REST API (v1). Cookie session; JSON content type on
// every request satisfies the server's CSRF guard.

export type User = { id: string; email: string; name: string };
export type Project = { id: string; key: string; name: string };
export type Status = "todo" | "in_progress" | "done";
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
};
export type SprintState = "planned" | "active" | "closed";
export type Sprint = {
  id: string;
  project_key: string;
  name: string;
  goal: string;
  state: SprintState;
  started_at: string | null;
  completed_at: string | null;
};
export type NotificationKind = "assigned" | "commented" | "mentioned";
export type Notification = {
  id: string;
  kind: NotificationKind;
  issue_key: string;
  issue_title: string;
  actor_name: string;
  excerpt: string;
  read: boolean;
  created_at: string;
};
export type Inbox = { items: Notification[]; unread: number };
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
export type Completion = { sprint: Sprint; completed: number; returned: number };
export type IssuePatch = {
  version: number;
  title?: string;
  description?: string;
  priority?: Priority;
  assignee_id?: string | null;
  sprint_id?: string | null;
  estimate?: number | null;
};
export type Comment = { id: string; author_id: string; author_name: string; body: string; created_at: string };
export type Role = "admin" | "member" | "viewer";
export type Member = { user_id: string; email: string; name: string; role: Role };

export const STATUSES: { id: Status; label: string }[] = [
  { id: "todo", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

export const ISSUE_TYPES: IssueType[] = ["task", "story", "bug", "epic", "subtask"];
export const PRIORITIES: Priority[] = ["highest", "high", "medium", "low", "lowest"];

/** RFC 9457 problem details as returned by the API (application/problem+json). */
export type Problem = { type: string; title: string; status: number; code: string; detail?: string; numeric_code?: number };

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    /** Stable Kyber error code (e.g. ISSUE_CONFLICT) — branch on this, not on message text. */
    readonly code: string = "",
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function isProblem(v: unknown): v is Problem {
  return typeof v === "object" && v !== null && "title" in v && typeof v.title === "string";
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  const data: unknown = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    if (isProblem(data)) throw new ApiError(res.status, data.detail || data.title, data.code);
    throw new ApiError(res.status, `request failed (${res.status})`);
  }
  return data as T;
}

const seg = encodeURIComponent;
type List<T> = { items: T[] };

export const api = {
  me: () => request<User>("GET", "/api/v1/me"),
  signup: (email: string, name: string, password: string) =>
    request<User>("POST", "/api/v1/auth/signup", { email, name, password }),
  login: (email: string, password: string) => request<{ token: string }>("POST", "/api/v1/auth/login", { email, password }),
  logout: () => request<undefined>("POST", "/api/v1/auth/logout"),

  projects: () => request<List<Project>>("GET", "/api/v1/projects").then((r) => r.items),
  createProject: (key: string, name: string) => request<Project>("POST", "/api/v1/projects", { key, name }),
  members: (key: string) => request<List<Member>>("GET", `/api/v1/projects/${seg(key)}/members`).then((r) => r.items),
  setMember: (key: string, email: string, role: Role) =>
    request<undefined>("POST", `/api/v1/projects/${seg(key)}/members`, { email, role }),

  /** Issues in backlog rank order; `sprint` filters to a sprint id or "backlog". */
  issues: (key: string, sprint?: string) =>
    request<List<Issue>>("GET", `/api/v1/projects/${seg(key)}/issues${sprint ? `?sprint=${seg(sprint)}` : ""}`).then((r) => r.items),
  rankIssue: (issueKey: string, anchor: { after: string } | { before: string }) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/rank`, anchor),
  sprints: (key: string) => request<List<Sprint>>("GET", `/api/v1/projects/${seg(key)}/sprints`).then((r) => r.items),
  createSprint: (key: string, name: string, goal: string) =>
    request<Sprint>("POST", `/api/v1/projects/${seg(key)}/sprints`, { name, goal }),
  startSprint: (id: string, endsAt?: string) =>
    request<Sprint>("POST", `/api/v1/sprints/${seg(id)}/start`, endsAt ? { ends_at: endsAt } : undefined),
  reportSummary: (key: string) => request<ReportSummary>("GET", `/api/v1/projects/${seg(key)}/reports/summary`),
  reportCreatedVsResolved: (key: string, days: number) =>
    request<{ days: DayStat[] }>("GET", `/api/v1/projects/${seg(key)}/reports/created-vs-resolved?days=${days}`),
  reportVelocity: (key: string) => request<Velocity>("GET", `/api/v1/projects/${seg(key)}/reports/velocity`),
  burndown: (sprintId: string) => request<Burndown>("GET", `/api/v1/sprints/${seg(sprintId)}/burndown`),
  notifications: () => request<Inbox>("GET", "/api/v1/notifications"),
  markNotificationRead: (id: string) => request<undefined>("POST", `/api/v1/notifications/${seg(id)}/read`),
  markAllNotificationsRead: () => request<undefined>("POST", "/api/v1/notifications/read-all"),
  completeSprint: (id: string) => request<Completion>("POST", `/api/v1/sprints/${seg(id)}/complete`),
  createIssue: (key: string, title: string, type: IssueType) =>
    request<Issue>("POST", `/api/v1/projects/${seg(key)}/issues`, { title, type }),
  transition: (issueKey: string, to: Status) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/transitions`, { to }),
  issue: (issueKey: string) => request<Issue>("GET", `/api/v1/issues/${seg(issueKey)}`),
  editIssue: (issueKey: string, patch: IssuePatch) => request<Issue>("PATCH", `/api/v1/issues/${seg(issueKey)}`, patch),
  comments: (issueKey: string) => request<List<Comment>>("GET", `/api/v1/issues/${seg(issueKey)}/comments`).then((r) => r.items),
  addComment: (issueKey: string, body: string) =>
    request<Comment>("POST", `/api/v1/issues/${seg(issueKey)}/comments`, { body }),
};
