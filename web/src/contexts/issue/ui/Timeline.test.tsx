import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Issue } from "../application";
import { IssueDetails } from "./IssueDetails";
import { Timeline } from "./Timeline";

const base: Issue = {
  id: "", key: "", title: "", type: "task", status: "todo", description: "", priority: "medium",
  assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 1,
};

describe("timeline", () => {
  it("draws epics and their issues as bars and lists unscheduled work", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/issues": {
        status: 200,
        body: {
          items: [
            { ...base, id: "1", key: "KYB-1", title: "Payments", type: "epic", start_date: "2026-10-01", due_date: "2026-10-10" },
            { ...base, id: "2", key: "KYB-2", title: "Card form", parent_key: "KYB-1", due_date: "2026-10-05" },
            { ...base, id: "3", key: "KYB-3", title: "Someday" },
          ],
        },
      },
    });
    const onOpen = vi.fn();
    renderWithProviders(<Timeline projectKey="KYB" onOpen={onOpen} />);
    expect(await screen.findByRole("img", { name: "KYB-1: 2026-10-01 → 2026-10-10" })).toHaveStyle({ width: "100%" });
    expect(screen.getByRole("img", { name: "KYB-2: 2026-10-05" })).toHaveStyle({ marginLeft: "40%", width: "10%" });
    expect(screen.getByText("Unscheduled (1)")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /KYB-2/ }));
    expect(onOpen).toHaveBeenCalledWith("KYB-2");
  });

  it("explains an empty timeline", async () => {
    mockApi({ "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [{ ...base, key: "KYB-1" }] } } });
    renderWithProviders(<Timeline projectKey="KYB" onOpen={() => {}} />);
    expect(await screen.findByText(/Nothing is scheduled yet/)).toBeInTheDocument();
  });

  it("shows a loading failure", async () => {
    mockApi({ "GET /api/v1/projects/KYB/issues": { status: 500, body: { type: "about:blank", title: "t", status: 500, code: "INTERNAL", detail: "x" } } });
    renderWithProviders(<Timeline projectKey="KYB" onOpen={() => {}} />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("sets an issue's start and due dates from its details", async () => {
    const issue: Issue = { ...base, id: "i-1", key: "KYB-1", version: 3, due_date: "2026-10-20" };
    const calls = mockApi({
      "GET /api/v1/projects/KYB/fields": { status: 200, body: { items: [] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...issue, start_date: "2026-10-13", version: 4 } },
    });
    renderWithProviders(<IssueDetails issue={issue} projectKey="KYB" />);
    expect(screen.getByLabelText("Due date")).toHaveValue("2026-10-20");
    await userEvent.type(screen.getByLabelText("Start date"), "2026-10-13");
    await userEvent.click(screen.getByRole("button", { name: "Save details" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({ version: 3, custom_fields: {}, start_date: "2026-10-13" }),
    );
  });
});
