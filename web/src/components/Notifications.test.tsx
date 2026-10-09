import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AppRoutes } from "../App";
import { mockApi, renderWithProviders } from "../test-utils";
import { IssuePanel } from "./IssuePanel";

const me = { id: "u-bob", email: "bob@x.uz", name: "Bob" };
const note = (id: string, kind: string, read = false) => ({
  id, kind, issue_key: "KYB-1", issue_title: "Login page", actor_name: "Alice", excerpt: kind === "assigned" ? "" : "@bob@x.uz look",
  read, created_at: "2026-04-01T10:00:00Z",
});
const base = {
  "GET /api/v1/me": { status: 200, body: me },
  "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p", key: "KYB", name: "Kyber" }] } },
};

describe("Notifications (KYB-S24)", () => {
  it("AC1: the bell shows the unread count", async () => {
    mockApi({ ...base, "GET /api/v1/notifications": { status: 200, body: { items: [note("n1", "assigned"), note("n2", "mentioned")], unread: 2 } } });
    renderWithProviders(<AppRoutes />, "/");
    expect(await screen.findByRole("button", { name: "Notifications (2 unread)" })).toBeInTheDocument();
  });

  it("AC2: lists notifications; clicking one marks it read and opens the issue", async () => {
    const calls = mockApi({
      ...base,
      "GET /api/v1/notifications": { status: 200, body: { items: [note("n1", "mentioned")], unread: 1 } },
      "POST /api/v1/notifications/n1/read": { status: 204 },
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
      "GET /api/v1/issues/KYB-1": { status: 200, body: { id: "i", key: "KYB-1", title: "Login page", type: "task", status: "todo", description: "", priority: "medium", assignee_id: null, reporter_id: null, sprint_id: null, rank: "a0", version: 1 } },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AppRoutes />, "/");
    await userEvent.click(await screen.findByRole("button", { name: /Notifications/ }));
    const panel = screen.getByRole("dialog", { name: "Notifications" });
    const item = within(panel).getByRole("button", { name: /Alice mentioned you on KYB-1/ });
    expect(item).toHaveTextContent("@bob@x.uz look");
    await userEvent.click(item);
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/v1/notifications/n1/read")).toBe(true));
    expect(await screen.findByRole("dialog", { name: /KYB-1/ })).toBeInTheDocument();
  });

  it("AC2: mark all as read", async () => {
    let unread = 2;
    const calls = mockApi({
      ...base,
      "GET /api/v1/notifications": () => ({ status: 200, body: { items: [note("n1", "assigned", unread === 0), note("n2", "commented", unread === 0)], unread } }),
      "POST /api/v1/notifications/read-all": () => {
        unread = 0;
        return { status: 204 };
      },
    });
    renderWithProviders(<AppRoutes />, "/");
    await userEvent.click(await screen.findByRole("button", { name: "Notifications (2 unread)" }));
    await userEvent.click(screen.getByRole("button", { name: "Mark all as read" }));
    expect(await screen.findByRole("button", { name: "Notifications" })).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/v1/notifications/read-all")).toBe(true);
  });

  it("shows an empty state", async () => {
    mockApi({ ...base, "GET /api/v1/notifications": { status: 200, body: { items: [], unread: 0 } } });
    renderWithProviders(<AppRoutes />, "/");
    await userEvent.click(await screen.findByRole("button", { name: "Notifications" }));
    expect(screen.getByText("You're all caught up.")).toBeInTheDocument();
  });
});

describe("Issue panel people (KYB-S21 AC2, S24 AC3)", () => {
  it("shows the reporter and hints @email mentions", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { id: "i", key: "KYB-1", title: "Login", type: "task", status: "todo", description: "", priority: "medium", assignee_id: null, reporter_id: "u-ali", sprint_id: null, rank: "a0", version: 1 } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-ali", email: "ali@x.uz", name: "Ali Valiyev", role: "admin" }] } },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    expect(await screen.findByText("Reporter: Ali Valiyev")).toBeInTheDocument();
    expect(screen.getByLabelText("Add a comment")).toHaveAttribute("placeholder", expect.stringContaining("@email"));
  });
});
