import { describe, expect, it } from "vitest";
import { initials, type Issue, parseLabels, planTimeline, canTransition, layoutWorkflow, toggleTransition, type Workflow } from "./model";

const base: Issue = {
  id: "", key: "", title: "", type: "task", status: "todo", description: "", priority: "medium",
  assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 1,
};
const mk = (key: string, extra: Partial<Issue> = {}): Issue => ({ ...base, id: key, key, title: key, ...extra });

describe("planTimeline", () => {
  it("spans the scheduled issues and nests them under their epics", () => {
    const plan = planTimeline([
      mk("KYB-3", { parent_key: "KYB-1", start_date: "2026-10-05", due_date: "2026-10-07" }),
      mk("KYB-1", { type: "epic", start_date: "2026-10-01", due_date: "2026-10-10" }),
      mk("KYB-4", { due_date: "2026-10-12" }),
      mk("KYB-2", { parent_key: "KYB-1" }),
      mk("KYB-5"),
    ]);
    expect(plan.from).toBe("2026-10-01");
    expect(plan.days).toBe(12);
    expect(plan.rows.map((r) => [r.issue.key, r.depth, r.offset, r.span])).toEqual([
      ["KYB-1", 0, 0, 10],
      ["KYB-3", 1, 4, 3],
      ["KYB-2", 1, null, null],
      ["KYB-4", 0, 11, 1],
    ]);
    expect(plan.unscheduled.map((i) => i.key)).toEqual(["KYB-5"]);
  });

  it("is empty when nothing is scheduled", () => {
    const plan = planTimeline([mk("KYB-1")]);
    expect(plan.days).toBe(0);
    expect(plan.rows).toEqual([]);
    expect(plan.unscheduled).toHaveLength(1);
  });
});

describe("initials", () => {
  it.each([
    ["Ali Valiyev", "AV"],
    ["  bob  ", "B"],
    ["Ana Maria Lopez", "AL"],
    ["", "?"],
  ])("%s → %s", (name, want) => expect(initials(name)).toBe(want));
});

describe("parseLabels", () => {
  it.each([
    ["ui backend ui", ["backend", "ui"]],
    ["  a,b  c ", ["a", "b", "c"]],
    ["", []],
  ])("%s", (raw, want) => expect(parseLabels(raw)).toEqual(want));
});

describe("workflow transitions (visual editor)", () => {
  const wf: Workflow = {
    initial: "todo",
    statuses: [
      { id: "todo", name: "To Do", category: "todo" },
      { id: "doing", name: "Doing", category: "in_progress" },
      { id: "done", name: "Done", category: "done" },
    ],
    transitions: [],
  };
  it("treats no transitions as any-to-any", () => {
    expect(canTransition(wf, "todo", "done")).toBe(true);
    expect(canTransition(wf, "todo", "todo")).toBe(false);
  });
  it("turning one transition off makes the rest explicit", () => {
    const off = toggleTransition(wf, "todo", "done");
    expect(canTransition(off, "todo", "done")).toBe(false);
    expect(off.transitions).toHaveLength(5);
    const on = toggleTransition(off, "todo", "done");
    expect(canTransition(on, "todo", "done")).toBe(true);
    expect(on.transitions).toHaveLength(6);
  });
  it("lays statuses out in category columns", () => {
    const pos = layoutWorkflow({ ...wf, statuses: [...wf.statuses, { id: "review", name: "Review", category: "in_progress" }] });
    expect(pos.get("todo")!.x).toBeLessThan(pos.get("doing")!.x);
    expect(pos.get("doing")!.x).toBe(pos.get("review")!.x);
    expect(pos.get("review")!.y).toBeGreaterThan(pos.get("doing")!.y);
    expect(pos.get("done")!.x).toBeGreaterThan(pos.get("review")!.x);
  });
});
