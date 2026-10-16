// The demo on GitHub Pages has no Go server: DemoServer answers the API in the browser from memory, and
// a storage (localStorage in the browser) keeps the visitor's changes across reloads.
//
// Projects, issues, comments, sprints, workflows and search follow the real rules closely. Every other
// resource goes through one generic store: POST /<parent>/<id>/<things> creates a thing under its parent,
// GET lists them, PUT/PATCH/DELETE /<things>/<id> edit and remove one, POST /<things>/<id>/<action> moves
// its state. That keeps goals, docs, forms, releases, meetings, chat and the rest working end to end.
import type { Sprint } from "../../contexts/agile/application";
import type { ReportSummary } from "../../contexts/insights/application";
import { type Comment, DEFAULT_WORKFLOW, type Issue, type IssueType, type Priority, type Workflow } from "../../contexts/issue/application";
import type { Member, Project } from "../../contexts/project/application";
import type { SearchHit } from "../../contexts/search/application";

export type DemoResponse = { status: number; body?: unknown };
export type DemoStorage = { load(): unknown; save(state: unknown): void };

type DemoIssue = Issue & { project_key: string };
type Item = Record<string, unknown> & { id: string };
type State = {
  v: 1;
  seq: number;
  projects: Project[];
  members: Record<string, Member[]>;
  issues: DemoIssue[];
  comments: Record<string, Comment[]>;
  sprints: Sprint[];
  workflows: Record<string, Workflow>;
  /** The generic store: things by kind ("goals", "messages", …), each with its parent path in _parent. */
  things: Record<string, Item[]>;
  /** Settings written with PUT to a path that is not a thing (prefs, status, sections). */
  docs: Record<string, unknown>;
};

const ME = { id: "u-demo", email: "demo@kyber.dev", name: "Demo User" };
const PEOPLE: Member[] = [
  { user_id: ME.id, email: ME.email, name: ME.name, role: "admin" },
  { user_id: "u-aziza", email: "aziza@kyber.dev", name: "Aziza Karimova", role: "member" },
  { user_id: "u-bekzod", email: "bekzod@kyber.dev", name: "Bekzod Tursunov", role: "member" },
];

const SEED: [string, IssueType, string, Priority, string | null, number | null][] = [
  ["Login page crashes on Safari", "bug", "in_progress", "highest", "u-aziza", 3],
  ["Board: drag cards between columns", "story", "done", "high", ME.id, 5],
  ["Write onboarding guide", "task", "todo", "medium", "u-bekzod", 2],
  ["Sprint burndown chart", "story", "in_progress", "medium", ME.id, 8],
  ["Email notifications for mentions", "story", "todo", "high", null, 5],
  ["Fix typo on the login screen", "bug", "done", "low", "u-aziza", 1],
  ["JQL autocomplete for labels", "task", "todo", "medium", "u-bekzod", 3],
  ["Dark theme contrast pass", "task", "in_progress", "low", ME.id, 2],
  ["Import issues from Jira CSV", "epic", "todo", "high", null, null],
];

/** Kinds created through another path than their own (POST /channels makes a conversation). */
const KIND_ALIAS: Record<string, string> = { channels: "conversations", dms: "conversations" };
/** The field a child carries for its parent: /projects/KYB/goals → project_key "KYB". */
const PARENT_FIELD: Record<string, string> = {
  projects: "project_key", conversations: "conversation_id", issues: "issue_key", docs: "page_id",
  whiteboards: "whiteboard_id", messages: "thread_id", forms: "form_id", meetings: "meeting_id",
};
/** POST /<things>/<id>/<action> moves state; anything not listed here creates a child thing. */
const ACTIONS: Record<string, (now: string) => Record<string, unknown>> = {
  start: (now) => ({ state: "active", started_at: now }),
  complete: (now) => ({ state: "completed", completed_at: now }),
  release: (now) => ({ state: "released", released_at: now }),
  unrelease: () => ({ state: "unreleased", released_at: null }),
  archive: () => ({ state: "archived", archived: true }),
  unarchive: () => ({ state: "unreleased", archived: false }),
  join: () => ({ member: true }),
  leave: () => ({ member: false }),
  pin: (now) => ({ pinned_by: ME.id, pinned_at: now }),
  save: () => ({ saved: true }),
  read: () => ({ unread: 0 }),
  stop: (now) => ({ state: "stopped", ended_at: now }),
  pause: () => ({ state: "paused" }),
  resume: () => ({ state: "running" }),
  finish: (now) => ({ state: "completed", ended_at: now }),
  rotate: () => ({}),
  typing: () => ({}),
  cursor: () => ({}),
  presence: () => ({}),
  verify: () => ({ verified: true }),
  response: () => ({}),
};
/** Fields a new thing of a kind starts with, so the UI finds every field it reads. */
function defaults(kind: string, now: string): Record<string, unknown> {
  switch (kind) {
    case "conversations":
      return { kind: "channel", name: "", topic: "", private: false, member_ids: PEOPLE.map((p) => p.user_id), member: true, unread: 0, owner_id: ME.id, admin_ids: [ME.id], archived: false, project_key: "" };
    case "messages":
    case "replies":
      return { author_id: ME.id, body: "", edited_at: null, deleted: false, thread_id: null, reply_count: 0, reactions: [], issues: [], kind: "", file: null, pinned_by: null, pinned_at: null };
    case "goals":
      return { owner_id: ME.id, target_date: null, jql: "", status: "on_track", done: 0, total: 0, percent: 0 };
    case "docs":
      return { title: "", body: "", author_id: ME.id, updated_by: ME.id, updated_at: now };
    case "forms":
      return { name: "", intro: "", issue_type: "task", questions: [] };
    case "versions":
      return { name: "", description: "", release_date: null, state: "unreleased", released_at: null, issues: 0, done: 0, unresolved: 0 };
    case "meetings":
      return { title: "", organizer_id: ME.id, attendee_ids: [ME.id], responses: {}, scope: "personal" };
    case "announcements":
      return { title: "", body: "", author_id: ME.id };
    case "whiteboards":
      return { title: "", created_by: ME.id, elements: {} };
    case "boards":
      return { name: "", kind: "scrum", columns: [], kanban_backlog: false };
    case "fields":
      return { name: "", type: "text", options: [] };
    case "worklogs":
      return { author_id: ME.id, author_name: ME.name, started: now, seconds: 0, spent: "", comment: "" };
    case "filters":
      return { owner_id: ME.id, name: "", jql: "", shared: false, mine: true };
    default:
      return { author_id: ME.id, author_name: ME.name, created_by: ME.id };
  }
}

const problem = (status: number, code: string, detail: string): DemoResponse => ({
  status,
  body: { type: "about:blank", title: detail, status, code, detail },
});
const ok = (body: unknown, status = 200): DemoResponse => ({ status, body });
const noContent: DemoResponse = { status: 204 };
const strip = (item: Item) => Object.fromEntries(Object.entries(item).filter(([k]) => k !== "_parent"));

/** Things the UI reads as single objects: unknown ones are a 404, not an empty list. */
const OBJECT_GETS = /\/(reports\/[a-z-]+|permissions|summary|burndown|report|notification-settings|access|prefs|call|status)(\?|$)/;

export class DemoServer {
  private s: State;

  constructor(
    private readonly clock: () => Date = () => new Date(),
    private readonly storage?: DemoStorage,
  ) {
    const saved = storage?.load() as State | undefined;
    this.s = saved && saved.v === 1 ? saved : this.seed();
  }

  handle(method: string, url: string, body?: unknown): DemoResponse {
    const res = this.route(method, url, body);
    if (method !== "GET" && res.status < 400) this.storage?.save(this.s);
    return res;
  }

  private route(method: string, url: string, body?: unknown): DemoResponse {
    const u = new URL(url, "http://demo.local");
    const path = u.pathname.replace(/^\/api\/v1/, "");
    const input = (body ?? {}) as Record<string, unknown>;
    const seg = path.split("/").filter(Boolean).map(decodeURIComponent);
    let m: RegExpMatchArray | null;

    if (path === "/me" && method === "GET") return ok(ME);
    if (path === "/auth/logout") return noContent;
    if (path === "/admin/access") return problem(403, "FORBIDDEN", "Administration is not part of the demo.");
    if (path === "/notifications" && method === "GET") return ok({ items: [], unread: 0 });
    if (path === "/me/notification-settings" && method === "GET")
      return ok({ email: false, telegram: false, email_available: false, telegram_available: false, telegram_connected: false, time_zone: "", ...(this.s.docs[path] as object) });
    if (path.startsWith("/focus")) return this.focus(method, seg[1], input);

    if (path === "/projects") return method === "GET" ? ok({ items: this.s.projects }) : this.createProject(input);
    if ((m = path.match(/^\/projects\/([^/]+)$/)) && method === "GET") {
      const p = this.s.projects.find((x) => x.key === m?.[1]);
      return p ? ok(p) : problem(404, "NOT_FOUND", "No such project.");
    }
    if ((m = path.match(/^\/projects\/([^/]+)\/members$/))) {
      const key = m[1] ?? "";
      if (method === "GET") return ok({ items: this.s.members[key] ?? [] });
      const person = PEOPLE.find((p) => p.email === input.email);
      if (!person) return problem(404, "USER_NOT_FOUND", "In the demo you can invite aziza@kyber.dev or bekzod@kyber.dev.");
      const list = (this.s.members[key] ??= []);
      if (!list.some((x) => x.user_id === person.user_id)) list.push({ ...person, role: (input.role as Member["role"]) ?? "member" });
      return ok(list.find((x) => x.user_id === person.user_id), 201);
    }
    if ((m = path.match(/^\/projects\/([^/]+)\/workflow$/))) {
      const key = m[1] ?? "";
      if (method === "GET") return ok(this.workflow(key));
      this.s.workflows[key] = input as unknown as Workflow;
      return ok(input);
    }
    if ((m = path.match(/^\/projects\/([^/]+)\/sprints$/))) {
      const key = m[1] ?? "";
      if (method === "GET") return ok({ items: this.s.sprints.filter((s) => s.project_key === key) });
      const sp: Sprint = { id: this.id("s"), project_key: key, name: String(input.name || `Sprint ${this.s.sprints.length + 1}`), goal: String(input.goal ?? ""), state: "planned", started_at: null, completed_at: null };
      this.s.sprints.push(sp);
      return ok(sp, 201);
    }
    if ((m = path.match(/^\/sprints\/([^/]+)\/(start|complete)$/))) return this.sprintStep(m[1] ?? "", m[2] ?? "", input);
    if ((m = path.match(/^\/projects\/([^/]+)\/labels$/)) && method === "GET")
      return ok({ items: [...new Set(this.s.issues.filter((i) => i.project_key === m?.[1]).flatMap((i) => i.labels ?? []))].sort() });
    if ((m = path.match(/^\/projects\/([^/]+)\/issues$/))) return this.projectIssues(method, m[1] ?? "", u.searchParams.get("sprint"), input);
    if ((m = path.match(/^\/projects\/([^/]+)\/reports\/summary$/))) return ok(this.summary(m[1] ?? ""));
    // Daily series start empty: the demo's issues all came to be in this browser.
    if (/^\/projects\/[^/]+\/reports\/(created-vs-resolved|cumulative-flow)$/.test(path)) return ok({ days: [] });
    if (/^\/projects\/[^/]+\/reports\/velocity$/.test(path)) return ok({ sprints: [] });
    if (/^\/projects\/[^/]+\/reports\/control-chart$/.test(path))
      return ok({ days: Number(u.searchParams.get("days") ?? 30), average_hours: 0, stddev_hours: 0, points: [] });
    if (/^\/projects\/[^/]+\/permissions$/.test(path) && method === "GET") return ok({ custom: false, permissions: [] });

    if ((m = path.match(/^\/issues\/([^/]+)(?:\/([a-z]+))?$/)) && this.s.issues.some((i) => i.key === m?.[1])) {
      const res = this.issue(method, m[1] ?? "", m[2] ?? "", input);
      if (res) return res;
    }
    if (path === "/search" && method === "GET") return ok(this.search(u.searchParams.get("jql") ?? ""));

    return this.generic(method, path, seg, input);
  }

  // ---- seed and helpers

  private seed(): State {
    const s: State = {
      v: 1, seq: 0,
      projects: [{ id: "p-kyb", key: "KYB", name: "Kyber demo" }],
      members: { KYB: [...PEOPLE] },
      issues: [], comments: {}, sprints: [], workflows: {}, things: {}, docs: {},
    };
    this.s = s;
    const now = this.iso();
    s.sprints.push({ id: "s-1", project_key: "KYB", name: "Sprint 1", goal: "Ship the board", state: "active", started_at: now, completed_at: null });
    for (const [title, type, status, priority, assignee, estimate] of SEED) {
      const issue = this.newIssue("KYB", { title, type, priority, assignee_id: assignee, estimate });
      Object.assign(issue, { status, resolution: status === "done" ? "done" : "", sprint_id: type === "epic" ? null : "s-1" });
    }
    s.comments["KYB-1"] = [{ id: "c-1", author_id: "u-aziza", author_name: "Aziza Karimova", body: "Reproduced on Safari 17. Looking into it.", created_at: now, reactions: [] }];
    s.things.conversations = [{ id: "conv-general", _parent: "", ...defaults("conversations", now), name: "general", topic: "Say hello 👋", created_at: now }];
    s.things.messages = [
      { id: "msg-1", _parent: "conversations/conv-general", ...defaults("messages", now), conversation_id: "conv-general", author_id: "u-aziza", body: "Welcome to the Kyber demo! Try the board, the backlog and the reports.", created_at: now },
    ];
    s.things.goals = [{ id: "goal-1", _parent: "projects/KYB", ...defaults("goals", now), project_key: "KYB", name: "Ship the board", jql: "project = KYB", total: 9, done: 2, percent: 22, version: 1 }];
    return s;
  }

  private iso(): string {
    return this.clock().toISOString();
  }

  private id(prefix: string): string {
    return `${prefix}-${++this.s.seq}`;
  }

  private workflow(key: string): Workflow {
    return this.s.workflows[key] ?? DEFAULT_WORKFLOW;
  }

  // ---- projects and issues

  private createProject(input: Record<string, unknown>): DemoResponse {
    const key = String(input.key ?? "").toUpperCase();
    if (!/^[A-Z][A-Z0-9]{1,9}$/.test(key)) return problem(400, "VALIDATION", "Key: 2–10 letters or digits, starting with a letter.");
    if (this.s.projects.some((p) => p.key === key)) return problem(409, "PROJECT_EXISTS", `Project ${key} already exists.`);
    const p = { id: `p-${key.toLowerCase()}`, key, name: String(input.name || key) };
    this.s.projects.push(p);
    this.s.members[key] = [PEOPLE[0] as Member];
    return ok(p, 201);
  }

  private projectIssues(method: string, key: string, sprint: string | null, input: Record<string, unknown>): DemoResponse {
    if (!this.s.projects.some((p) => p.key === key)) return problem(404, "NOT_FOUND", "No such project.");
    if (method === "GET") {
      const items = this.s.issues
        .filter((i) => i.project_key === key && (!sprint || (sprint === "backlog" ? !i.sprint_id : i.sprint_id === sprint)))
        .sort((a, b) => a.rank.localeCompare(b.rank));
      return ok({ items });
    }
    if (!String(input.title ?? "").trim()) return problem(400, "VALIDATION", "Title is required.");
    return ok(this.newIssue(key, input), 201);
  }

  private newIssue(projectKey: string, input: Record<string, unknown>): DemoIssue {
    const n = this.s.issues.filter((i) => i.project_key === projectKey).length + 1;
    const issue: DemoIssue = {
      id: `i-${projectKey}-${n}`, key: `${projectKey}-${n}`, project_key: projectKey,
      title: String(input.title ?? ""), type: (input.type as IssueType) ?? "task", status: this.workflow(projectKey).initial,
      description: String(input.description ?? ""), priority: (input.priority as Priority) ?? "medium",
      assignee_id: (input.assignee_id as string | null) ?? null, reporter_id: ME.id, estimate: (input.estimate as number | null) ?? null,
      sprint_id: (input.sprint_id as string | null) ?? null, rank: this.nextRank(), version: 1,
      parent_key: (input.parent_key as string | null) ?? null, resolution: "", labels: [], fix_versions: [], custom_fields: {},
      start_date: null, due_date: null,
    };
    this.s.issues.push(issue);
    return issue;
  }

  private nextRank(): string {
    return `r${String(this.s.issues.length + 1).padStart(6, "0")}`;
  }

  private issue(method: string, key: string, sub: string, input: Record<string, unknown>): DemoResponse | null {
    const issue = this.s.issues.find((i) => i.key === key) as DemoIssue;
    if (sub === "" && method === "GET") return ok(issue);
    if (sub === "" && method === "PATCH") {
      if (input.version !== undefined && input.version !== issue.version) return problem(409, "ISSUE_CONFLICT", "This issue was changed by someone else.");
      const { version: _v, ...changes } = input;
      Object.assign(issue, changes, { version: issue.version + 1 });
      return ok(issue);
    }
    if (sub === "" && method === "DELETE") {
      this.s.issues = this.s.issues.filter((i) => i !== issue);
      return noContent;
    }
    if (sub === "transitions" && method === "POST") {
      const to = String(input.to ?? "");
      const st = this.workflow(issue.project_key).statuses.find((s) => s.id === to);
      if (!st) return problem(400, "VALIDATION", "Unknown status.");
      Object.assign(issue, { status: to, resolution: st.category === "done" ? (input.resolution ?? "done") : "", version: issue.version + 1 });
      return ok(issue);
    }
    if (sub === "rank" && method === "POST") {
      const anchorKey = String(input.before ?? input.after ?? "");
      const list = this.s.issues.filter((i) => i.project_key === issue.project_key && i !== issue).sort((a, b) => a.rank.localeCompare(b.rank));
      const at = list.findIndex((i) => i.key === anchorKey);
      if (at < 0) return problem(400, "VALIDATION", "Unknown anchor issue.");
      list.splice(input.before ? at : at + 1, 0, issue);
      list.forEach((i, n) => (i.rank = `r${String(n + 1).padStart(6, "0")}`));
      issue.version++;
      return ok(issue);
    }
    if (sub === "comments") {
      const list = this.s.comments[key] ?? [];
      if (method === "GET") return ok({ items: list });
      const c: Comment = { id: this.id("c"), author_id: ME.id, author_name: ME.name, body: String(input.body ?? ""), created_at: this.iso(), reactions: [] };
      this.s.comments[key] = [...list, c];
      return ok(c, 201);
    }
    if (sub === "children" && method === "GET") return ok({ items: this.s.issues.filter((i) => i.parent_key === key) });
    return null; // links, worklogs, watchers…: the generic store
  }

  // ---- sprints and focus

  private sprintStep(id: string, step: string, input: Record<string, unknown>): DemoResponse {
    const sp = this.s.sprints.find((x) => x.id === id);
    if (!sp) return problem(404, "NOT_FOUND", "No such sprint.");
    if (step === "start") {
      if (this.s.sprints.some((x) => x.project_key === sp.project_key && x.state === "active")) return problem(409, "SPRINT_ACTIVE", "Another sprint is already active.");
      Object.assign(sp, { state: "active", started_at: this.iso() });
      return ok(sp);
    }
    const inSprint = this.s.issues.filter((i) => i.sprint_id === id);
    const doneIds = new Set(this.workflow(sp.project_key).statuses.filter((s) => s.category === "done").map((s) => s.id));
    const open = inSprint.filter((i) => !doneIds.has(i.status));
    for (const i of open) i.sprint_id = (input.move_to as string) || null;
    Object.assign(sp, { state: "closed", completed_at: this.iso() });
    return ok({ sprint: sp, completed: inSprint.length - open.length, returned: open.length });
  }

  private focus(method: string, action: string | undefined, input: Record<string, unknown>): DemoResponse {
    const now = this.clock();
    const cur = this.s.docs.focus as (Item & { planned_minutes: number; started_at: string; state: string }) | undefined;
    const view = (f: typeof cur) => {
      if (!f || f.state !== "running") return f ?? null;
      const left = Math.max(0, Date.parse(f.started_at) + f.planned_minutes * 60_000 - now.getTime());
      return { ...f, remaining_seconds: Math.round(left / 1000), remaining_ms: left, ends_at: new Date(now.getTime() + left).toISOString() };
    };
    if (method === "GET") return ok({ session: view(cur) });
    if (!action) {
      const f = { id: this.id("focus"), issue_key: String(input.issue_key ?? ""), user_id: ME.id, state: "running", planned_minutes: Number(input.minutes ?? 25), started_at: now.toISOString(), ended_at: null, focused_seconds: 0, remaining_seconds: Number(input.minutes ?? 25) * 60 };
      this.s.docs.focus = f;
      return ok(view(f), 201);
    }
    if (!cur) return problem(404, "NOT_FOUND", "No focus session.");
    const ended = { ...cur, ...(ACTIONS[action]?.(now.toISOString()) ?? {}), focused_seconds: Math.round((now.getTime() - Date.parse(cur.started_at)) / 1000) };
    this.s.docs.focus = ended.state === "running" || ended.state === "paused" ? ended : undefined;
    return ok(ended);
  }

  // ---- the generic store

  private generic(method: string, path: string, seg: string[], input: Record<string, unknown>): DemoResponse {
    const now = this.iso();
    const find = (kind: string, id: string) => (this.s.things[KIND_ALIAS[kind] ?? kind] ?? []).find((t) => t.id === id);
    const last = seg.at(-1) ?? "";
    // /<things>/<id>/<action>
    if (method === "POST" && seg.length === 3 && ACTIONS[last]) {
      const t = find(seg[0] ?? "", seg[1] ?? "");
      if (!t) return noContent;
      Object.assign(t, ACTIONS[last]?.(now));
      return ok(strip(t));
    }
    // /<things>/<id>
    if (seg.length === 2 && find(seg[0] ?? "", seg[1] ?? "")) {
      const kind = KIND_ALIAS[seg[0] ?? ""] ?? seg[0] ?? "";
      const t = find(kind, seg[1] ?? "") as Item;
      if (method === "GET") return ok(strip(t));
      if (method === "DELETE") {
        this.s.things[kind] = (this.s.things[kind] ?? []).filter((x) => x !== t);
        return noContent;
      }
      const { version: _v, ...changes } = input;
      Object.assign(t, changes, { version: Number(t.version ?? 1) + 1, updated_at: now });
      return ok(strip(t));
    }
    // a collection: /<things> or /<parent>/<id>/<things>
    if (seg.length === 1 || seg.length === 3) {
      const kind = KIND_ALIAS[last] ?? last;
      const parent = seg.length === 3 ? `${seg[0]}/${seg[1]}` : "";
      if (method === "GET" && !OBJECT_GETS.test(path)) {
        const items = (this.s.things[kind] ?? []).filter((t) => (seg.length === 1 ? true : t._parent === parent)).map(strip);
        return ok({ items });
      }
      if (method === "POST") {
        const parentField = seg.length === 3 ? PARENT_FIELD[seg[0] ?? ""] : undefined;
        const item: Item = {
          ...defaults(kind, now), id: this.id(kind.replace(/s$/, "")), _parent: parent, created_at: now, updated_at: now, version: 1,
          ...(parentField ? { [parentField]: seg[1] } : {}), ...input,
        };
        if (last === "dms") Object.assign(item, { kind: "dm", name: "", member_ids: [ME.id, ...((input.user_ids as string[]) ?? [])] });
        if (kind === "messages" || kind === "replies") Object.assign(item, { author_id: ME.id });
        (this.s.things[kind] ??= []).push(item);
        if (kind === "replies") {
          const parentMsg = find("messages", seg[1] ?? "");
          if (parentMsg) parentMsg.reply_count = Number(parentMsg.reply_count ?? 0) + 1;
        }
        return ok(strip(item), 201);
      }
    }
    if (method === "GET") {
      if (path in this.s.docs) return ok(this.s.docs[path]);
      return OBJECT_GETS.test(path) ? problem(404, "DEMO_UNSUPPORTED", "Not available in the demo.") : ok({ items: [] });
    }
    if (method === "PUT" || method === "PATCH") {
      this.s.docs[path] = { ...((this.s.docs[path] as object) ?? {}), ...input };
      return ok(this.s.docs[path]);
    }
    return noContent;
  }

  // ---- search and reports

  private search(jql: string): { total: number; items: SearchHit[] } {
    const project = jql.match(/project\s*=\s*"?(\w+)"?/i)?.[1]?.toUpperCase();
    const text = jql.match(/(?:summary|text)\s*~\s*"([^"]*)"/i)?.[1]?.toLowerCase();
    const type = jql.match(/type\s*=\s*"?(\w+)"?/i)?.[1]?.toLowerCase();
    const mine = /assignee\s*=\s*currentUser\(\)/i.test(jql);
    const unassigned = /assignee\s+is\s+empty/i.test(jql);
    const notDone = /statusCategory\s*!=\s*done|resolution\s+is\s+empty/i.test(jql);
    const items = this.s.issues
      .filter((i) => {
        const st = this.workflow(i.project_key).statuses.find((s) => s.id === i.status);
        return (!project || i.project_key === project) && (!text || i.title.toLowerCase().includes(text)) && (!type || i.type === type)
          && (!mine || i.assignee_id === ME.id) && (!unassigned || !i.assignee_id) && (!notDone || st?.category !== "done");
      })
      .map((i) => {
        const st = this.workflow(i.project_key).statuses.find((s) => s.id === i.status);
        return {
          key: i.key, project_key: i.project_key, title: i.title, type: i.type, status: i.status,
          status_name: st?.name ?? i.status, status_category: st?.category ?? "todo", priority: i.priority,
          assignee_id: i.assignee_id, resolution: i.resolution ?? "", points: i.estimate,
        };
      });
    return { total: items.length, items };
  }

  private summary(projectKey: string): ReportSummary {
    const list = this.s.issues.filter((i) => i.project_key === projectKey);
    const count = (key: (i: Issue) => string) => {
      const by = new Map<string, { key: string; count: number; points: number }>();
      for (const i of list) {
        const b = by.get(key(i)) ?? { key: key(i), count: 0, points: 0 };
        b.count++;
        b.points += i.estimate ?? 0;
        by.set(key(i), b);
      }
      return [...by.values()];
    };
    const doneIds = new Set(this.workflow(projectKey).statuses.filter((s) => s.category === "done").map((s) => s.id));
    const open = list.filter((i) => !doneIds.has(i.status));
    const people = this.s.members[projectKey] ?? [];
    return {
      total: list.length, open: open.length, done: list.length - open.length, unassigned: open.filter((i) => !i.assignee_id).length,
      total_points: list.reduce((n, i) => n + (i.estimate ?? 0), 0), open_points: open.reduce((n, i) => n + (i.estimate ?? 0), 0),
      by_status: count((i) => i.status), by_type: count((i) => i.type), by_priority: count((i) => i.priority),
      workload: count((i) => i.assignee_id ?? "").map((b) => ({ user_id: b.key || null, name: people.find((p) => p.user_id === b.key)?.name ?? "Unassigned", count: b.count, points: b.points })),
      cycle_time: { count: 0, average_hours: 0, median_hours: 0 },
    };
  }
}
