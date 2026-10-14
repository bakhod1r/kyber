import { describe, expect, it } from "vitest";
import { initials, type Issue, planTimeline } from "./model";

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
