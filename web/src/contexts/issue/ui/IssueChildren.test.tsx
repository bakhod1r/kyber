import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Issue } from "../application";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { IssueChildren } from "./IssueChildren";

const issue = (n: number, type: Issue["type"], extra: Partial<Issue> = {}): Issue => ({
  id: `i-${n}`, key: `KYB-${n}`, title: `Issue ${n}`, type, status: "todo", description: "", priority: "medium",
  assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 1, parent_key: null, ...extra,
});

describe("IssueChildren (KYB-S45)", () => {
  it("shows an epic's issues with progress and adds a child", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1/children": { status: 200, body: { items: [issue(2, "story", { status: "done" }), issue(3, "task")], total: 2, done: 1, points: 8, done_points: 5 } },
      "POST /api/v1/projects/KYB/issues": { status: 201, body: issue(4, "story", { parent_key: "KYB-1" }) },
    });
    const onOpen = vi.fn();
    renderWithProviders(<IssueChildren issue={issue(1, "epic")} projectKey="KYB" onOpen={onOpen} />);
    expect(await screen.findByRole("heading", { name: "Issues in this epic" })).toBeInTheDocument();
    expect(await screen.findByRole("progressbar", { name: "1 of 2 done" })).toHaveAttribute("aria-valuenow", "50");
    expect(screen.getByText("5 of 8 points done")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /KYB-3/ }));
    expect(onOpen).toHaveBeenCalledWith("KYB-3");
    await userEvent.type(screen.getByLabelText("New child title"), "Apple Pay");
    await userEvent.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ title: "Apple Pay", type: "story", parent_key: "KYB-1" }));
  });

  it("stories hold sub-tasks", async () => {
    mockApi({ "GET /api/v1/issues/KYB-2/children": { status: 200, body: { items: [], total: 0, done: 0, points: 0, done_points: 0 } } });
    renderWithProviders(<IssueChildren issue={issue(2, "story")} projectKey="KYB" />);
    expect(await screen.findByRole("heading", { name: "Sub-tasks" })).toBeInTheDocument();
    expect(await screen.findByText("No sub-tasks yet.")).toBeInTheDocument();
  });

  it("sub-tasks have no children", () => {
    const { container } = renderWithProviders(<IssueChildren issue={issue(5, "subtask", { parent_key: "KYB-2" })} projectKey="KYB" />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("IssuePanel parent link", () => {
  it("links a sub-task to its parent", async () => {
    const { IssuePanel } = await import("./IssuePanel");
    mockApi({
      "GET /api/v1/issues/KYB-5": { status: 200, body: issue(5, "subtask", { parent_key: "KYB-2" }) },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
      "GET /api/v1/issues/KYB-5/comments": { status: 200, body: { items: [] } },
    });
    const onOpen = vi.fn();
    renderWithProviders(<IssuePanel issueKey="KYB-5" projectKey="KYB" onClose={vi.fn()} onOpen={onOpen} />);
    await userEvent.click(await screen.findByRole("button", { name: "KYB-2" }));
    expect(onOpen).toHaveBeenCalledWith("KYB-2");
  });
});
