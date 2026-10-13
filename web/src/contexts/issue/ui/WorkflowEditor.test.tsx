import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { WorkflowEditor } from "./WorkflowEditor";

const simplified = {
  initial: "todo",
  statuses: [
    { id: "todo", name: "To Do", category: "todo" },
    { id: "in_progress", name: "In Progress", category: "in_progress" },
    { id: "done", name: "Done", category: "done" },
  ],
  transitions: [],
};

describe("WorkflowEditor (KYB-S43)", () => {
  it("adds a status and saves the workflow", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: simplified },
      "PUT /api/v1/projects/KYB/workflow": (init) => ({ status: 200, body: JSON.parse(String(init?.body)) }),
    });
    renderWithProviders(<WorkflowEditor projectKey="KYB" />);
    expect(await screen.findByLabelText("Status 2 name")).toHaveValue("In Progress");
    await userEvent.type(screen.getByLabelText("New status name"), "Code Review");
    await userEvent.selectOptions(screen.getByLabelText("New status category"), "in_progress");
    await userEvent.click(screen.getByRole("button", { name: "Add status" }));
    await userEvent.click(screen.getByRole("button", { name: "Save workflow" }));

    await waitFor(() => expect(calls.some((c) => c.key === "PUT /api/v1/projects/KYB/workflow")).toBe(true));
    const sent = calls.find((c) => c.key === "PUT /api/v1/projects/KYB/workflow")?.body as typeof simplified;
    expect(sent.statuses.map((s) => s.id)).toEqual(["todo", "in_progress", "code_review", "done"]);
    expect(sent.statuses[2]).toEqual({ id: "code_review", name: "Code Review", category: "in_progress" });
    expect(await screen.findByRole("status")).toHaveTextContent("Workflow saved");
  });

  it("removes a status and shows the server's refusal", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: simplified },
      "PUT /api/v1/projects/KYB/workflow": {
        status: 409,
        body: { type: "about:blank", title: "Issues still use this status. Move them first.", status: 409, code: "WORKFLOW_STATUS_IN_USE", detail: "in_progress (KYB-1)" },
      },
    });
    renderWithProviders(<WorkflowEditor projectKey="KYB" />);
    await userEvent.click(await screen.findByRole("button", { name: "Remove In Progress" }));
    expect(screen.getAllByRole("textbox", { name: /^Status \d name$/ }).map((i) => (i as HTMLInputElement).value)).toEqual(["To Do", "Done"]);
    await userEvent.click(screen.getByRole("button", { name: "Save workflow" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/in_progress \(KYB-1\)|Move them first/);
  });

  it("renames a status and changes the initial status", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: simplified },
      "PUT /api/v1/projects/KYB/workflow": (init) => ({ status: 200, body: JSON.parse(String(init?.body)) }),
    });
    renderWithProviders(<WorkflowEditor projectKey="KYB" />);
    const name = await screen.findByLabelText("Status 1 name");
    await userEvent.clear(name);
    await userEvent.type(name, "Backlog");
    await userEvent.selectOptions(screen.getByLabelText("Initial status"), "in_progress");
    await userEvent.click(screen.getByRole("button", { name: "Save workflow" }));
    await waitFor(() => expect(calls.some((c) => c.key.startsWith("PUT"))).toBe(true));
    const sent = calls.find((c) => c.key.startsWith("PUT"))?.body as typeof simplified;
    expect(sent.initial).toBe("in_progress");
    expect(sent.statuses[0]).toEqual({ id: "todo", name: "Backlog", category: "todo" });
  });
});
