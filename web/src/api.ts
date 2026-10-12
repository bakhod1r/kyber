// Typed client for the Kyber REST API (v1). Cookie session; JSON content type on
// every request satisfies the server's CSRF guard.

export type User = { id: string; email: string; name: string };
export type Project = { id: string; key: string; name: string };
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
  security_level?: "" | "confidential";
};
export type Comment = { id: string; author_id: string; author_name: string; body: string; created_at: string };
export type Role = "admin" | "member" | "viewer";
export type Member = { user_id: string; email: string; name: string; role: Role };

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
export type NotificationSettings = {
  email: boolean;
  telegram: boolean;
  email_available: boolean;
  telegram_available: boolean;
  telegram_connected: boolean;
};
export type AuditEntry = { at: string; actor_id: string; actor_name: string; action: string; target: string; status: number };
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
export type BoardColumn = { name: string; statuses: Status[]; min: number; max: number };
export type BoardView = { id: string; project_key: string; name: string; kind: "scrum" | "kanban"; columns: BoardColumn[]; version: number };

/** Jira's simplified workflow, used until the project's own workflow has loaded. */
export const DEFAULT_WORKFLOW: Workflow = {
  initial: "todo",
  statuses: STATUSES.map((s) => ({ id: s.id, name: s.label, category: s.id as StatusCategory })),
  transitions: [],
};

export type ImportItem = {
  line: number;
  external_key: string;
  issue_key: string | null;
  title: string;
  type: IssueType;
  status: Status;
  priority: Priority;
  assignee_id: string | null;
  points: number | null;
  created_at: string | null;
  resolved_at: string | null;
  warnings: string[];
};
export type ImportReport = {
  dry_run: boolean;
  items: ImportItem[];
  errors: { line: number; external_key: string; message: string }[];
  skipped: string[];
};

export type AuthProviders = { google: boolean; telegram_bot: string | null; telegram_otp?: boolean };
export type OtpChallenge = { id: string; link: string; expires_at: string };
/** The Telegram Login Widget's user object, forwarded unchanged for server-side verification. */
export type TelegramUser = { id: number; auth_date: number; hash: string; first_name?: string; last_name?: string; username?: string; photo_url?: string };

export type FocusState = "running" | "paused" | "completed" | "interrupted";
export type FocusSession = {
  id: string;
  issue_key: string;
  user_id: string;
  state: FocusState;
  planned_minutes: number;
  started_at: string;
  ended_at: string | null;
  focused_seconds: number;
  remaining_seconds: number;
  /** Exact time left (ms); absolute deadline while running; server clock at response time. */
  remaining_ms?: number;
  ends_at?: string | null;
  server_time?: string;
  reason: string;
};
export type Meeting = { id: string; title: string; starts_at: string; ends_at: string; organizer_id: string; attendee_ids: string[] };

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
  authProviders: () => request<AuthProviders>("GET", "/api/v1/auth/providers"),
  telegramOtpStart: () => request<OtpChallenge>("POST", "/api/v1/auth/telegram/otp", {}),
  telegramOtpVerify: (id: string, code: string) => request<{ token: string }>("POST", "/api/v1/auth/telegram/otp/verify", { id, code }),
  telegramLogin: (user: TelegramUser) => request<{ token: string }>("POST", "/api/v1/auth/telegram", user),
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
  /** Previews (dryRun) or runs a Jira CSV import; admins only. */
  importJira: (key: string, csv: string, dryRun: boolean) =>
    request<ImportReport>("POST", `/api/v1/projects/${seg(key)}/import/jira?dry_run=${dryRun}`, { csv }),
  focus: () => request<{ session: FocusSession | null }>("GET", "/api/v1/focus").then((r) => r.session),
  startFocus: (issueKey: string, minutes: number) => request<FocusSession>("POST", "/api/v1/focus", { issue_key: issueKey, minutes }),
  focusAction: (action: "pause" | "resume" | "stop") => request<FocusSession>("POST", `/api/v1/focus/${action}`),
  issueFocus: (issueKey: string) =>
    request<{ items: FocusSession[]; total_focused_seconds: number }>("GET", `/api/v1/issues/${seg(issueKey)}/focus`),
  meetings: (from?: string, to?: string) =>
    request<List<Meeting>>("GET", `/api/v1/meetings${from && to ? `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` : ""}`).then((r) => r.items),
  scheduleMeeting: (m: { title: string; starts_at: string; ends_at: string; attendee_ids: string[] }) =>
    request<Meeting>("POST", "/api/v1/meetings", m),
  cancelMeeting: (id: string) => request<undefined>("DELETE", `/api/v1/meetings/${seg(id)}`),
  reportSummary: (key: string) => request<ReportSummary>("GET", `/api/v1/projects/${seg(key)}/reports/summary`),
  reportCreatedVsResolved: (key: string, days: number) =>
    request<{ days: DayStat[] }>("GET", `/api/v1/projects/${seg(key)}/reports/created-vs-resolved?days=${days}`),
  reportVelocity: (key: string) => request<Velocity>("GET", `/api/v1/projects/${seg(key)}/reports/velocity`),
  burndown: (sprintId: string) => request<Burndown>("GET", `/api/v1/sprints/${seg(sprintId)}/burndown`),
  notifications: () => request<Inbox>("GET", "/api/v1/notifications"),
  markNotificationRead: (id: string) => request<undefined>("POST", `/api/v1/notifications/${seg(id)}/read`),
  markAllNotificationsRead: () => request<undefined>("POST", "/api/v1/notifications/read-all"),
  completeSprint: (id: string) => request<Completion>("POST", `/api/v1/sprints/${seg(id)}/complete`),
  createIssue: (key: string, title: string, type: IssueType, parentKey?: string) =>
    request<Issue>("POST", `/api/v1/projects/${seg(key)}/issues`, parentKey ? { title, type, parent_key: parentKey } : { title, type }),
  links: (issueKey: string) => request<List<IssueLink>>("GET", `/api/v1/issues/${seg(issueKey)}/links`).then((r) => r.items),
  link: (issueKey: string, type: LinkType, toKey: string) =>
    request<IssueLink>("POST", `/api/v1/issues/${seg(issueKey)}/links`, { type, to_key: toKey }),
  unlink: (id: string) => request<undefined>("DELETE", `/api/v1/links/${seg(id)}`),
  children: (issueKey: string) =>
    request<{ items: Issue[]; total: number; done: number; points: number; done_points: number }>("GET", `/api/v1/issues/${seg(issueKey)}/children`),
  notificationSettings: () => request<NotificationSettings>("GET", "/api/v1/me/notification-settings"),
  setNotificationSettings: (patch: Partial<Pick<NotificationSettings, "email" | "telegram">>) =>
    request<NotificationSettings>("PUT", "/api/v1/me/notification-settings", patch),
  connectTelegram: () => request<{ url: string; expires_at: string }>("POST", "/api/v1/me/telegram/connect"),
  telegramGroups: (key: string) =>
    request<List<{ chat_id: string; title: string; linked_at: string }>>("GET", `/api/v1/projects/${seg(key)}/telegram-groups`).then((r) => r.items),
  telegramGroupLink: (key: string) =>
    request<{ url: string; expires_at: string }>("POST", `/api/v1/projects/${seg(key)}/telegram-groups/link`),
  unlinkTelegramGroup: (key: string, chat: string) =>
    request<undefined>("DELETE", `/api/v1/projects/${seg(key)}/telegram-groups/${seg(chat)}`),
  audit: (key: string) => request<List<AuditEntry>>("GET", `/api/v1/projects/${seg(key)}/audit`).then((r) => r.items),
  history: (issueKey: string) => request<List<HistoryEntry>>("GET", `/api/v1/issues/${seg(issueKey)}/history`).then((r) => r.items),
  boards: (key: string) => request<List<BoardView>>("GET", `/api/v1/projects/${seg(key)}/boards`).then((r) => r.items),
  workflow: (key: string) => request<Workflow>("GET", `/api/v1/projects/${seg(key)}/workflow`),
  setWorkflow: (key: string, wf: Workflow) => request<Workflow>("PUT", `/api/v1/projects/${seg(key)}/workflow`, wf),
  transition: (issueKey: string, to: Status) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/transitions`, { to }),
  issue: (issueKey: string) => request<Issue>("GET", `/api/v1/issues/${seg(issueKey)}`),
  editIssue: (issueKey: string, patch: IssuePatch) => request<Issue>("PATCH", `/api/v1/issues/${seg(issueKey)}`, patch),
  comments: (issueKey: string) => request<List<Comment>>("GET", `/api/v1/issues/${seg(issueKey)}/comments`).then((r) => r.items),
  addComment: (issueKey: string, body: string) =>
    request<Comment>("POST", `/api/v1/issues/${seg(issueKey)}/comments`, { body }),
};
