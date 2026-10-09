import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { Reports } from "./ReportsPage";

const summary = {
  total: 12, open: 9, done: 3, unassigned: 2, total_points: 34.5, open_points: 21,
  by_status: [{ key: "todo", count: 6, points: 13 }, { key: "in_progress", count: 3, points: 8 }, { key: "done", count: 3, points: 13.5 }],
  by_type: [{ key: "story", count: 5, points: 20 }, { key: "bug", count: 7, points: 14.5 }],
  by_priority: [{ key: "high", count: 4, points: 10 }, { key: "medium", count: 8, points: 24.5 }],
  workload: [{ user_id: "u-1", name: "Ali Valiyev", count: 5, points: 13 }, { user_id: null, name: "Unassigned", count: 2, points: 3 }],
  cycle_time: { count: 3, average_hours: 52, median_hours: 48 },
};
const days = (n: number) => ({
  days: Array.from({ length: n }, (_, i) => ({
    date: new Date(Date.UTC(2026, 4, 1 + i)).toISOString().slice(0, 10), created: i % 3, resolved: i % 2, cum_created: 10 + i, cum_resolved: 5 + i,
  })),
});
const sprint = { id: "s-1", project_key: "KYB", name: "Sprint 7", goal: "Reports", state: "active", started_at: "2026-05-04T09:00:00Z", ends_at: "2026-05-18T09:00:00Z", completed_at: null };
const burndown = {
  sprint: { id: "s-1", name: "Sprint 7", state: "active", started_at: "2026-05-04T09:00:00Z", ends_at: "2026-05-18T09:00:00Z", completed_at: null },
  start_points: 10, start_issues: 3, added_points: 8, added_issues: 1, removed_issues: 0,
  samples: [
    { at: "2026-05-04T09:00:00Z", remaining_points: 10, remaining_issues: 3 },
    { at: "2026-05-06T09:00:00Z", remaining_points: 7, remaining_issues: 2 },
    { at: "2026-05-07T09:00:00Z", remaining_points: 15, remaining_issues: 3 },
  ],
  ideal: [{ at: "2026-05-04T09:00:00Z", remaining_points: 10, remaining_issues: 3 }, { at: "2026-05-18T09:00:00Z", remaining_points: 0, remaining_issues: 0 }],
};
const velocity = { sprints: [{ id: "s-0", name: "Sprint 6", committed_points: 20, completed_points: 17, completed_issues: 6 }] };

function api(extra = {}) {
  return mockApi({
    "GET /api/v1/projects/KYB/reports/summary": { status: 200, body: summary },
    "GET /api/v1/projects/KYB/reports/created-vs-resolved?days=30": { status: 200, body: days(30) },
    "GET /api/v1/projects/KYB/reports/created-vs-resolved?days=90": { status: 200, body: days(90) },
    "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint] } },
    "GET /api/v1/sprints/s-1/burndown": { status: 200, body: burndown },
    "GET /api/v1/projects/KYB/reports/velocity": { status: 200, body: velocity },
    ...extra,
  });
}

describe("Reports (KYB-S28)", () => {
  it("AC1: KPI tiles show the headline numbers", async () => {
    api();
    renderWithProviders(<Reports projectKey="KYB" />);
    const kpis = await screen.findByRole("region", { name: "Key figures" });
    expect(within(kpis).getByText("Open issues").nextSibling).toHaveTextContent("9");
    expect(within(kpis).getByText("Done").nextSibling).toHaveTextContent("3");
    expect(within(kpis).getByText("Open story points").nextSibling).toHaveTextContent("21");
    expect(within(kpis).getByText("Unassigned").nextSibling).toHaveTextContent("2");
    expect(within(kpis).getByText("Median cycle time").nextSibling).toHaveTextContent("2.0 days");
  });

  it("AC1/AC2: every chart is a labelled figure with a data table", async () => {
    api();
    renderWithProviders(<Reports projectKey="KYB" />);
    for (const name of ["Issues by status", "Issues by type", "Issues by priority", "Workload (open issues)", "Created vs resolved", "Burndown — Sprint 7", "Velocity"]) {
      const fig = await screen.findByRole("figure", { name });
      expect(within(fig).getByRole("table", { hidden: true })).toBeInTheDocument();
    }
    const status = screen.getByRole("figure", { name: "Issues by status" });
    const rows = within(status).getAllByRole("row", { hidden: true }).map((r) => r.textContent);
    expect(rows).toContain("In Progress38");
    const burn = screen.getByRole("figure", { name: "Burndown — Sprint 7" });
    expect(within(burn).getByText(/Scope added after start: 8 points \(1 issue\)/)).toBeInTheDocument();
  });

  it("AC1: the date range filter refetches created vs resolved", async () => {
    const calls = api();
    renderWithProviders(<Reports projectKey="KYB" />);
    await screen.findByRole("figure", { name: "Created vs resolved" });
    await userEvent.selectOptions(screen.getByLabelText("Date range"), "90");
    await waitFor(() => expect(calls.some((c) => c.key.endsWith("days=90"))).toBe(true));
  });

  it("shows empty states when there is no sprint data", async () => {
    api({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/reports/velocity": { status: 200, body: { sprints: [] } },
    });
    renderWithProviders(<Reports projectKey="KYB" />);
    expect(await screen.findByText("Start a sprint to see its burndown.")).toBeInTheDocument();
    expect(screen.getByText("Complete a sprint to see velocity.")).toBeInTheDocument();
  });
});
