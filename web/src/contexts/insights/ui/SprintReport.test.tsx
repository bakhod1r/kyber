import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { SprintReport } from "./SprintReport";

const sp = (id: string, name: string, state: string) => ({
  id, project_key: "KYB", name, goal: "", state, started_at: "2026-10-01T09:00:00Z", ends_at: "2026-10-14T09:00:00Z", completed_at: null,
});
const item = (key: string, points: number, added = false) => ({ key, title: `Title ${key}`, type: "task", points, status_category: "todo", added_after_start: added });
const report = (name: string, state: string) => ({
  sprint: { id: "x", name, state, started_at: null, ends_at: null, completed_at: null },
  committed_points: 10, committed_issues: 3, added_points: 8, added_issues: 1, completed_points: 3,
  completed: [item("KYB-1", 3)], not_completed: [item("KYB-2", 5), item("KYB-4", 8, true)], removed: [],
});

describe("SprintReport (S47)", () => {
  it("shows the active sprint, and a closed one on request", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sp("s-1", "Sprint 1", "closed"), sp("s-2", "Sprint 2", "active"), sp("s-3", "Sprint 3", "planned")] } },
      "GET /api/v1/sprints/s-2/report": { status: 200, body: report("Sprint 2", "active") },
      "GET /api/v1/sprints/s-1/report": { status: 200, body: report("Sprint 1", "closed") },
    });
    renderWithProviders(<SprintReport projectKey="KYB" />);
    const notYet = await screen.findByRole("list", { name: "Not completed yet" });
    expect(within(notYet).getByText("Title KYB-4")).toBeInTheDocument();
    expect(within(notYet).getByTitle("Added after the sprint started")).toBeInTheDocument();
    expect(screen.getAllByText("None.")).toHaveLength(1); // nothing removed
    expect(screen.queryByRole("option", { name: /Sprint 3/ })).not.toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Sprint"), "s-1");
    expect(await screen.findByRole("list", { name: "Not completed (carried over)" })).toBeInTheDocument();
  });

  it("asks to start a sprint first, and shows errors", async () => {
    mockApi({ "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sp("s-3", "Sprint 3", "planned")] } } });
    const { unmount } = renderWithProviders(<SprintReport projectKey="KYB" />);
    expect(await screen.findByText("Start a sprint to see its report.")).toBeInTheDocument();
    unmount();
    mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sp("s-2", "Sprint 2", "active")] } },
      "GET /api/v1/sprints/s-2/report": { status: 404, body: { type: "about:blank", title: "t", status: 404, code: "SPRINT_NOT_FOUND", detail: "gone" } },
    });
    renderWithProviders(<SprintReport projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});
