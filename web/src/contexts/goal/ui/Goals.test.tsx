import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Goal } from "../application";
import { Goals } from "./Goals";

const goal: Goal = {
  id: "g-1", project_key: "KYB", owner_id: "u-1", name: "Zero bugs", target_date: "2099-01-01", jql: "type = bug",
  status: "on_track", done: 1, total: 4, percent: 25, version: 1,
};
const invalid = { type: "about:blank", title: "t", status: 422, code: "SEARCH_INVALID_JQL", detail: "JQL at 5: expected a value" };

describe("goals", () => {
  it("adds a goal, reports its status and deletes it", async () => {
    let items: Goal[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/goals": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/goals": () => {
        items = [goal];
        return { status: 201, body: goal };
      },
      "PUT /api/v1/goals/g-1": () => {
        items = [{ ...goal, status: "at_risk", version: 2 }];
        return { status: 200, body: items[0] };
      },
      "DELETE /api/v1/goals/g-1": () => {
        items = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<Goals projectKey="KYB" />);
    expect(await screen.findByText(/No goals yet/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Goal name"), "Zero bugs");
    await userEvent.type(screen.getByLabelText("Target date"), "2099-01-01");
    await userEvent.type(screen.getByLabelText("Issues (JQL)"), "type = bug");
    await userEvent.click(screen.getByRole("button", { name: "Add goal" }));
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/goals")?.body).toEqual({ name: "Zero bugs", target_date: "2099-01-01", jql: "type = bug" });
    expect(await screen.findByRole("progressbar", { name: "Progress of Zero bugs" })).toHaveAttribute("aria-valuenow", "25");
    expect(screen.getByText(/1 of 4 issues done/)).toBeInTheDocument();
    expect(screen.getByText(/days left/)).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Status of Zero bugs"), "at_risk");
    await waitFor(() =>
      expect(calls.find((c) => c.key === "PUT /api/v1/goals/g-1")?.body).toEqual({ version: 1, name: "Zero bugs", target_date: "2099-01-01", jql: "type = bug", status: "at_risk" }),
    );
    await userEvent.click(await screen.findByRole("button", { name: "Delete goal Zero bugs" }));
    expect(await screen.findByText(/No goals yet/)).toBeInTheDocument();
  });

  it("shows goals without issues or a target date, and query errors", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/goals": { status: 200, body: { items: [{ ...goal, jql: "", target_date: null, done: 0, total: 0, percent: 0 }] } },
      "POST /api/v1/projects/KYB/goals": { status: 422, body: invalid },
      "PUT /api/v1/goals/g-1": { status: 422, body: invalid },
    });
    renderWithProviders(<Goals projectKey="KYB" />);
    expect(await screen.findByText("No issues linked.")).toBeInTheDocument();
    expect(screen.getByText("No target date")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Goal name"), "Broken");
    await userEvent.click(screen.getByRole("button", { name: "Add goal" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await userEvent.selectOptions(screen.getByLabelText("Status of Zero bugs"), "done");
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("shows a loading failure", async () => {
    mockApi({ "GET /api/v1/projects/KYB/goals": { status: 404, body: { ...invalid, status: 404, code: "PROJECT_NOT_FOUND", detail: "x" } } });
    renderWithProviders(<Goals projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});
