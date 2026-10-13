import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { ProjectSummary } from "./ProjectSummary";

describe("ProjectSummary", () => {
  it("shows the project at a glance", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/reports/summary": {
        status: 200,
        body: {
          total: 5, open: 3, done: 2, unassigned: 1, total_points: 10, open_points: 6,
          by_status: [{ key: "todo", count: 3, points: 6 }, { key: "done", count: 2, points: 4 }],
          by_type: [{ key: "bug", count: 2, points: 0 }], by_priority: [{ key: "high", count: 1, points: 0 }],
          workload: [{ user_id: "u", name: "Ali", count: 2, points: 3 }], cycle_time: { count: 0, average_hours: 0, median_hours: 0 },
        },
      },
      "GET /api/v1/projects/KYB/reports/created-vs-resolved?days=7": {
        status: 200, body: { days: [{ date: "2026-10-13", created: 4, resolved: 1, cum_created: 4, cum_resolved: 1 }] },
      },
      "GET /api/v1/projects/KYB/versions": {
        status: 200,
        body: { items: [
          { id: "v", project_key: "KYB", name: "1.0", description: "", release_date: "2026-12-01T00:00:00Z", state: "unreleased", released_at: null, version: 1, issues: 4, done: 1 },
          { id: "w", project_key: "KYB", name: "0.9", description: "", release_date: null, state: "released", released_at: null, version: 1 },
        ] },
      },
    });
    renderWithProviders(<ProjectSummary projectKey="KYB" />);
    const week = await screen.findByRole("region", { name: "Last 7 days" });
    expect(within(week).getByText("Created").parentElement).toHaveTextContent("4");
    expect(screen.getByRole("figure", { name: "Status overview" })).toBeInTheDocument();
    const releases = screen.getByRole("region", { name: "Upcoming releases" });
    expect(await within(releases).findByText("1.0")).toBeInTheDocument();
    expect(within(releases).queryByText("0.9")).not.toBeInTheDocument();
  });

  it("explains errors and an empty roadmap", async () => {
    mockApi({ "GET /api/v1/projects/KYB/reports/summary": { status: 404, body: { type: "about:blank", title: "t", status: 404, code: "PROJECT_NOT_FOUND", detail: "gone" } } });
    renderWithProviders(<ProjectSummary projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
