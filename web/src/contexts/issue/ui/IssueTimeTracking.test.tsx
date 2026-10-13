import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Issue } from "../application";
import { IssueTimeTracking } from "./IssueTimeTracking";

const issue: Issue = {
  id: "i-1", key: "KYB-1", title: "Parser", type: "task", status: "todo", description: "", priority: "medium",
  assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 2,
  time_tracking: { original_seconds: 28800, remaining_seconds: 18000, spent_seconds: 10800, original: "1d", remaining: "5h", spent: "3h" },
};

describe("IssueTimeTracking", () => {
  it("sets the estimate, logs work and deletes a worklog", async () => {
    let items = [{ id: "w-1", issue_key: "KYB-1", author_id: "u-1", author_name: "Ali", started: "2026-10-14T09:00:00Z", seconds: 10800, spent: "3h", comment: "lexer" }];
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1/worklogs": () => ({ status: 200, body: { items } }),
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...issue, version: 3 } },
      "POST /api/v1/issues/KYB-1/worklogs": { status: 201, body: items[0] },
      "DELETE /api/v1/worklogs/w-1": () => {
        items = [];
        return { status: 204 };
      },
      "GET /api/v1/issues/KYB-1": { status: 200, body: issue },
    });
    renderWithProviders(<IssueTimeTracking issue={issue} />);
    expect(screen.getByText(/Logged 3h · Remaining 5h · Estimate 1d/)).toBeInTheDocument();
    expect(await screen.findByText(/lexer/)).toBeInTheDocument();

    await userEvent.clear(screen.getByLabelText("Original estimate"));
    await userEvent.type(screen.getByLabelText("Original estimate"), "2d");
    await userEvent.click(screen.getByRole("button", { name: "Set estimate" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({ version: 2, original_estimate: "2d" }));

    await userEvent.type(screen.getByLabelText("Time spent"), "1h 30m");
    await userEvent.type(screen.getByLabelText("Worklog comment"), "tests");
    await userEvent.click(screen.getByRole("button", { name: "Log work" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/issues/KYB-1/worklogs")?.body).toEqual({ spent: "1h 30m", comment: "tests" }));

    await userEvent.click(screen.getByRole("button", { name: "Delete worklog 3h by Ali" }));
    await waitFor(() => expect(screen.queryByText(/lexer/)).not.toBeInTheDocument());
  });

  it("shows a rejected duration", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1/worklogs": { status: 200, body: { items: [] } },
      "POST /api/v1/issues/KYB-1/worklogs": { status: 422, body: { type: "about:blank", title: "t", status: 422, code: "VALIDATION_FAILED", detail: "invalid duration" } },
    });
    renderWithProviders(<IssueTimeTracking issue={{ ...issue, time_tracking: undefined }} />);
    expect(screen.getByText(/Estimate none/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Time spent"), "ages");
    await userEvent.click(screen.getByRole("button", { name: "Log work" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("invalid duration");
  });
});
