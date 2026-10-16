import { describe, expect, it } from "vitest";
import { DemoServer } from "./demoServer";

const now = () => new Date("2026-10-16T09:00:00Z");

describe("DemoServer (the in-browser API behind the GitHub Pages demo)", () => {
  it("signs everyone in as the demo user and seeds a project with issues", () => {
    const s = new DemoServer(now);
    expect(s.handle("GET", "/api/v1/me")).toMatchObject({ status: 200, body: { name: "Demo User" } });
    const projects = s.handle("GET", "/api/v1/projects").body as { items: { key: string }[] };
    expect(projects.items.map((p) => p.key)).toEqual(["KYB"]);
    const issues = s.handle("GET", "/api/v1/projects/KYB/issues").body as { items: unknown[] };
    expect(issues.items.length).toBeGreaterThan(5);
  });

  it("creates projects and issues with the next key", () => {
    const s = new DemoServer(now);
    expect(s.handle("POST", "/api/v1/projects", { key: "OPS", name: "Operations" }).status).toBe(201);
    const a = s.handle("POST", "/api/v1/projects/OPS/issues", { title: "First", type: "task" });
    const b = s.handle("POST", "/api/v1/projects/OPS/issues", { title: "Second", type: "bug" });
    expect([a.status, (a.body as { key: string }).key, (b.body as { key: string }).key]).toEqual([201, "OPS-1", "OPS-2"]);
    expect(s.handle("POST", "/api/v1/projects", { key: "OPS", name: "Again" }).status).toBe(409);
  });

  it("moves an issue across the board and edits it with optimistic locking", () => {
    const s = new DemoServer(now);
    const moved = s.handle("POST", "/api/v1/issues/KYB-1/transitions", { to: "done" });
    expect(moved.body).toMatchObject({ status: "done", resolution: "done" });
    const v = (moved.body as { version: number }).version;
    expect(s.handle("PATCH", "/api/v1/issues/KYB-1", { version: v, title: "Renamed" }).body).toMatchObject({ title: "Renamed", version: v + 1 });
    expect(s.handle("PATCH", "/api/v1/issues/KYB-1", { version: v, title: "Stale" })).toMatchObject({ status: 409, body: { code: "ISSUE_CONFLICT" } });
  });

  it("keeps comments per issue", () => {
    const s = new DemoServer(now);
    s.handle("POST", "/api/v1/issues/KYB-2/comments", { body: "Looks good" });
    const list = s.handle("GET", "/api/v1/issues/KYB-2/comments").body as { items: { body: string; author_name: string }[] };
    expect(list.items.at(-1)).toMatchObject({ body: "Looks good", author_name: "Demo User" });
  });

  it("answers JQL search with the project's issues, matching summary text", () => {
    const s = new DemoServer(now);
    const all = s.handle("GET", `/api/v1/search?jql=${encodeURIComponent("project = KYB")}`).body as { total: number };
    const some = s.handle("GET", `/api/v1/search?jql=${encodeURIComponent('summary ~ "login"')}`).body as { items: { title: string }[] };
    expect(all.total).toBeGreaterThan(5);
    expect(some.items.length).toBeGreaterThan(0);
    expect(some.items.every((i) => /login/i.test(i.title))).toBe(true);
  });

  it("summarises the project for reports", () => {
    const s = new DemoServer(now);
    const r = s.handle("GET", "/api/v1/projects/KYB/reports/summary").body as { total: number; by_status: { key: string; count: number }[] };
    expect(r.total).toBe(r.by_status.reduce((n, b) => n + b.count, 0));
    expect(s.handle("GET", "/api/v1/projects/KYB/reports/cumulative-flow?days=30").body).toEqual({ days: [] });
    expect(s.handle("GET", "/api/v1/projects/KYB/reports/velocity").body).toEqual({ sprints: [] });
    expect(s.handle("GET", "/api/v1/projects/KYB/reports/burndown").status).toBe(404);
  });

  it("returns an empty list for other collections and a problem for anything else", () => {
    const s = new DemoServer(now);
    expect(s.handle("GET", "/api/v1/projects/KYB/forms")).toEqual({ status: 200, body: { items: [] } });
    expect(s.handle("GET", "/api/v1/admin/access")).toMatchObject({ status: 403 });
    expect(s.handle("GET", "/api/v1/projects/KYB/reports/velocity").body).toEqual({ sprints: [] });
  });

  it("keeps any other collection: create under a project, list it, edit and delete it by id", () => {
    const s = new DemoServer(now);
    const before = (s.handle("GET", "/api/v1/projects/KYB/goals").body as { items: unknown[] }).items.length;
    const g = s.handle("POST", "/api/v1/projects/KYB/goals", { name: "Launch v1", jql: "project = KYB" });
    expect(g).toMatchObject({ status: 201, body: { name: "Launch v1", project_key: "KYB", status: "on_track", version: 1 } });
    const id = (g.body as { id: string }).id;
    expect((s.handle("GET", "/api/v1/projects/KYB/goals").body as { items: unknown[] }).items).toHaveLength(before + 1);
    expect(s.handle("PUT", `/api/v1/goals/${id}`, { name: "Launch 1.0" }).body).toMatchObject({ name: "Launch 1.0", version: 2 });
    expect(s.handle("DELETE", `/api/v1/goals/${id}`).status).toBe(204);
    expect((s.handle("GET", "/api/v1/projects/KYB/goals").body as { items: unknown[] }).items).toHaveLength(before);
  });

  it("runs a sprint: create, start, complete with unfinished issues back to the backlog", () => {
    const s = new DemoServer(now);
    s.handle("POST", "/api/v1/sprints/s-1/complete");
    const sp = s.handle("POST", "/api/v1/projects/KYB/sprints", { name: "Sprint 2", goal: "" }).body as { id: string; state: string };
    expect(sp.state).toBe("planned");
    expect(s.handle("POST", `/api/v1/sprints/${sp.id}/start`).body).toMatchObject({ state: "active" });
    const backlog = s.handle("GET", "/api/v1/projects/KYB/issues?sprint=backlog").body as { items: { status: string }[] };
    expect(backlog.items.some((i) => i.status !== "done")).toBe(true);
  });

  it("orders issues by rank and moves one before another", () => {
    const s = new DemoServer(now);
    s.handle("POST", "/api/v1/issues/KYB-3/rank", { before: "KYB-1" });
    const keys = (s.handle("GET", "/api/v1/projects/KYB/issues").body as { items: { key: string }[] }).items.map((i) => i.key);
    expect(keys.indexOf("KYB-3")).toBe(keys.indexOf("KYB-1") - 1);
  });

  it("chats: a channel with messages", () => {
    const s = new DemoServer(now);
    const c = s.handle("POST", "/api/v1/channels", { name: "general" }).body as { id: string; kind: string; member: boolean };
    expect(c).toMatchObject({ kind: "channel", member: true });
    s.handle("POST", `/api/v1/conversations/${c.id}/messages`, { body: "Hello team" });
    const msgs = s.handle("GET", `/api/v1/conversations/${c.id}/messages`).body as { items: { body: string; author_id: string }[] };
    expect(msgs.items).toMatchObject([{ body: "Hello team", author_id: "u-demo" }]);
    expect((s.handle("GET", "/api/v1/conversations").body as { items: unknown[] }).items.length).toBeGreaterThan(1);
  });

  it("saves the project's workflow and follows it on transitions", () => {
    const s = new DemoServer(now);
    const wf = { initial: "todo", statuses: [{ id: "todo", name: "To Do", category: "todo" }, { id: "review", name: "Review", category: "in_progress" }, { id: "done", name: "Done", category: "done" }], transitions: [] };
    s.handle("PUT", "/api/v1/projects/KYB/workflow", wf);
    expect(s.handle("GET", "/api/v1/projects/KYB/workflow").body).toEqual(wf);
    expect(s.handle("POST", "/api/v1/issues/KYB-3/transitions", { to: "review" }).body).toMatchObject({ status: "review" });
  });

  it("restores its state from storage, so a reload keeps the visitor's changes", () => {
    let saved: unknown;
    const storage = { load: () => saved, save: (st: unknown) => (saved = st) };
    const a = new DemoServer(now, storage);
    a.handle("POST", "/api/v1/projects/KYB/issues", { title: "Survives reload" });
    const b = new DemoServer(now, storage);
    const titles = (b.handle("GET", "/api/v1/projects/KYB/issues").body as { items: { title: string }[] }).items.map((i) => i.title);
    expect(titles).toContain("Survives reload");
  });

  it("serves the default permission scheme, like the server", () => {
    const s = new DemoServer(now);
    const scheme = s.handle("GET", "/api/v1/projects/KYB/permissions").body as { permissions: { permission: string }[] };
    expect(scheme.permissions.map((p) => p.permission)).toContain("CREATE_ISSUES");
  });
});
