// Typed client for the Kyber REST API (v1). Cookie session; JSON content type on
// every request satisfies the server's CSRF guard.

export type User = { id: string; email: string; name: string };
export type Project = { id: string; key: string; name: string };
export type Status = "todo" | "in_progress" | "done";
export type IssueType = "epic" | "story" | "task" | "bug" | "subtask";
export type Issue = { id: string; key: string; title: string; type: IssueType; status: Status };
export type Role = "admin" | "member" | "viewer";
export type Member = { user_id: string; email: string; name: string; role: Role };

export const STATUSES: { id: Status; label: string }[] = [
  { id: "todo", label: "To Do" },
  { id: "in_progress", label: "In Progress" },
  { id: "done", label: "Done" },
];

export const ISSUE_TYPES: IssueType[] = ["task", "story", "bug", "epic", "subtask"];

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

  issues: (key: string) => request<List<Issue>>("GET", `/api/v1/projects/${seg(key)}/issues`).then((r) => r.items),
  createIssue: (key: string, title: string, type: IssueType) =>
    request<Issue>("POST", `/api/v1/projects/${seg(key)}/issues`, { title, type }),
  transition: (issueKey: string, to: Status) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/transitions`, { to }),
};
