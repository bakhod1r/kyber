import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { BulkEdit } from "./BulkEdit";

const workflow = { initial: "todo", statuses: [{ id: "todo", name: "To Do", category: "todo" }, { id: "done", name: "Done", category: "done" }], transitions: [] };

describe("BulkEdit (G9)", () => {
  it("applies the chosen changes and reports the issues that failed", async () => {
    const done = vi.fn();
    const calls = mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-1", email: "a@x.uz", name: "Aziza", role: "admin" }] } },
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [{ id: "s-1", project_key: "KYB", name: "Sprint 1", goal: "", state: "active", version: 1 }] } },
      "POST /api/v1/projects/KYB/issues/bulk": {
        status: 200,
        body: { results: [{ key: "KYB-1", ok: true }, { key: "KYB-2", ok: false, code: "ISSUE_TRANSITION_NOT_ALLOWED", error: "transition not allowed" }] },
      },
    });
    renderWithProviders(<BulkEdit projectKey="KYB" keys={["KYB-1", "KYB-2"]} onDone={done} />);
    expect(await screen.findByText("2 issues selected")).toBeInTheDocument();
    await userEvent.selectOptions(await screen.findByLabelText("Status"), "done");
    await userEvent.selectOptions(screen.getByLabelText("Assignee"), "u-1");
    await userEvent.selectOptions(screen.getByLabelText("Sprint"), "s-1");
    await userEvent.type(screen.getByLabelText("Add labels"), "q4 urgent");
    await userEvent.click(screen.getByRole("button", { name: "Apply to 2 issues" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({
        keys: ["KYB-1", "KYB-2"], status: "done", assignee_id: "u-1", sprint_id: "s-1", add_labels: ["q4", "urgent"],
      }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("1 changed, 1 failed");
    expect(screen.getByText(/KYB-2: transition not allowed/)).toBeInTheDocument();
    expect(done).not.toHaveBeenCalled(); // failures stay on screen
  });

  it("sends only what was chosen and closes when everything worked", async () => {
    const done = vi.fn();
    const calls = mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/issues/bulk": { status: 200, body: { results: [{ key: "KYB-1", ok: true }] } },
    });
    renderWithProviders(<BulkEdit projectKey="KYB" keys={["KYB-1"]} onDone={done} />);
    const apply = await screen.findByRole("button", { name: "Apply to 1 issue" });
    expect(apply).toBeDisabled(); // nothing chosen yet
    await userEvent.selectOptions(screen.getByLabelText("Assignee"), "__none");
    await userEvent.click(apply);
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ keys: ["KYB-1"], assignee_id: "" }));
    await waitFor(() => expect(done).toHaveBeenCalled());
  });
});
