import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Issue } from "../application";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { Board } from "../../agile/ui/Board";
import { IssuePanel } from "./IssuePanel";

const base: Issue = {
  id: "i-1", key: "KYB-1", title: "Login page", type: "task", status: "todo",
  description: "Steps", priority: "medium", assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a1", version: 3,
};
const members = {
  items: [
    { user_id: "u-ali", email: "ali@x.uz", name: "Ali Valiyev", role: "admin" },
    { user_id: "u-bob", email: "bob@x.uz", name: "Bob", role: "member" },
  ],
};
const problem = (status: number, code: string, detail: string) => ({ type: "about:blank", title: "t", status, code, detail });

describe("IssuePanel (KYB-S17)", () => {
  it("AC1+AC2: edits fields and saves with the loaded version", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: base },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "PATCH /api/v1/issues/KYB-1": () => ({ status: 200, body: { ...base, title: "Login v2", priority: "high", assignee_id: "u-bob", version: 4 } }),
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const dialog = await screen.findByRole("dialog", { name: /KYB-1/ });
    const title = await within(dialog).findByLabelText("Title");
    await waitFor(() => expect(title).toHaveValue("Login page"));
    await userEvent.clear(title);
    await userEvent.type(title, "Login v2");
    await userEvent.selectOptions(within(dialog).getByLabelText("Priority"), "high");
    await userEvent.selectOptions(within(dialog).getByLabelText("Assignee"), "u-bob");
    await userEvent.click(within(dialog).getByLabelText(/Confidential/));
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    await waitFor(() => expect(calls.some((c) => c.key === "PATCH /api/v1/issues/KYB-1")).toBe(true));
    expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({
      version: 3, title: "Login v2", description: "Steps", priority: "high", assignee_id: "u-bob", estimate: null,
      security_level: "confidential",
      fix_versions: [],
      labels: [],
    });
    expect(await within(dialog).findByText("Saved")).toBeInTheDocument();
  });

  it("AC2: a 409 conflict explains and reloads the latest version", async () => {
    let version = 3;
    mockApi({
      "GET /api/v1/issues/KYB-1": () => ({ status: 200, body: { ...base, version, title: version === 3 ? "Login page" : "Changed by Bob" } }),
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "PATCH /api/v1/issues/KYB-1": () => {
        version = 4;
        return { status: 409, body: problem(409, "ISSUE_CONFLICT", "issue was modified concurrently") };
      },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const dialog = await screen.findByRole("dialog");
    await waitFor(async () => expect(await within(dialog).findByLabelText("Title")).toHaveValue("Login page"));
    await userEvent.type(within(dialog).getByLabelText("Title"), "!");
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(await within(dialog).findByRole("alert")).toHaveTextContent("changed by someone else");
    await waitFor(() => expect(within(dialog).getByLabelText("Title")).toHaveValue("Changed by Bob"));
  });

  it("AC3: lists and adds comments", async () => {
    const comments = [{ id: "c-1", author_id: "u-ali", author_name: "Ali Valiyev", body: "First!", created_at: "2026-01-01T10:00:00Z", reactions: [] }];
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: base },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": () => ({ status: 200, body: { items: comments } }),
      "POST /api/v1/issues/KYB-1/comments": () => {
        comments.push({ id: "c-2", author_id: "u-bob", author_name: "Bob", body: "On it", created_at: "2026-01-01T11:00:00Z", reactions: [] });
        return { status: 201, body: comments[1] };
      },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const dialog = await screen.findByRole("dialog");
    expect(await within(dialog).findByText("First!")).toBeInTheDocument();
    await userEvent.type(within(dialog).getByLabelText("Add a comment"), "On it");
    await userEvent.click(within(dialog).getByRole("button", { name: "Comment" }));
    expect(await within(dialog).findByText("On it")).toBeInTheDocument();
    expect(calls.find((c) => c.key === "POST /api/v1/issues/KYB-1/comments")?.body).toEqual({ body: "On it" });
  });

  it("closes on Escape and on the close button", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: base },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
    });
    const onClose = vi.fn();
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={onClose} />);
    await screen.findByRole("dialog");
    await userEvent.keyboard("{Escape}");
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});

describe("Board cards (KYB-S17 AC4)", () => {
  it("show priority and assignee initials, and open the panel on click", async () => {
    const onOpen = vi.fn();
    mockApi({
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [{ ...base, priority: "highest", assignee_id: "u-ali" }] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
    });
    renderWithProviders(<Board projectKey="KYB" onOpen={onOpen} />);
    const card = (await screen.findByText("Login page")).closest("article")!;
    expect(within(card).getByLabelText("Priority: highest")).toBeInTheDocument();
    expect(await within(card).findByTitle("Ali Valiyev")).toHaveTextContent("AV");
    await userEvent.click(within(card).getByRole("button", { name: /Login page/ }));
    expect(onOpen).toHaveBeenCalledWith("KYB-1");
  });
});

describe("Story points in the issue panel (KYB-S28 AC3)", () => {
  it("edits the estimate", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { ...base, estimate: 3 } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...base, estimate: 5, version: 4 } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const sp = await screen.findByLabelText("Story points");
    await waitFor(() => expect(sp).toHaveValue(3));
    await userEvent.clear(sp);
    await userEvent.type(sp, "5");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toMatchObject({ version: 3, estimate: 5 }));
  });

  it("S48: a resolved issue's resolution can be changed; an open one shows none", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { ...base, status: "done", resolution: "done" } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...base, status: "done", resolution: "wont_do", version: 4 } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const select = await screen.findByLabelText("Resolution");
    await userEvent.selectOptions(select, "wont_do");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toMatchObject({ resolution: "wont_do" }));
  });

  it("S48: an unresolved issue has no resolution field", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { ...base, resolution: "" } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    await screen.findByLabelText("Title");
    expect(screen.queryByLabelText("Resolution")).not.toBeInTheDocument();
  });

  it("S48: plans the issue into fix versions; archived ones are offered only when already set", async () => {
    const v = (id: string, name: string, state: string) => ({ id, project_key: "KYB", name, description: "", release_date: null, state, released_at: null, version: 1 });
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { ...base, fix_versions: ["v-old"] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/versions": { status: 200, body: { items: [v("v-1", "1.0", "unreleased"), v("v-old", "0.9", "archived"), v("v-x", "0.1", "archived")] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...base, version: 4 } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const group = await screen.findByRole("group", { name: "Fix versions" });
    expect(within(group).queryByLabelText("0.1")).not.toBeInTheDocument();
    await userEvent.click(within(group).getByLabelText("1.0"));
    await userEvent.click(within(group).getByLabelText("0.9"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toMatchObject({ fix_versions: ["v-1"] }));
  });

  it("labels the issue, suggesting the project's labels", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: { ...base, labels: ["ui"] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/labels": { status: 200, body: { items: ["backend", "ui"] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...base, version: 4 } },
    });
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const input = await screen.findByLabelText("Labels");
    expect(input).toHaveValue("ui");
    await waitFor(() => expect(document.querySelector('datalist option[value="backend"]')).not.toBeNull());
    await userEvent.type(input, " backend");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toMatchObject({ labels: ["backend", "ui"] }));
  });
});

describe("IssuePanel focus (a11y)", () => {
  const api = () =>
    mockApi({
      "GET /api/v1/issues/KYB-1": { status: 200, body: base },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "GET /api/v1/issues/KYB-1/comments": { status: 200, body: { items: [] } },
    });

  it("moves focus into the dialog and keeps Tab inside it", async () => {
    api();
    renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    const dialog = await screen.findByRole("dialog", { name: /KYB-1/ });
    await within(dialog).findByLabelText("Title");
    expect(dialog.contains(document.activeElement)).toBe(true);
    await userEvent.tab({ shift: true });
    expect(dialog.contains(document.activeElement)).toBe(true);
    for (let i = 0; i < 60; i++) await userEvent.tab();
    expect(dialog.contains(document.activeElement)).toBe(true);
  });

  it("gives focus back to what opened it when it closes", async () => {
    api();
    const opener = document.createElement("button");
    document.body.appendChild(opener);
    opener.focus();
    const { unmount } = renderWithProviders(<IssuePanel issueKey="KYB-1" projectKey="KYB" onClose={vi.fn()} />);
    await screen.findByRole("dialog", { name: /KYB-1/ });
    unmount();
    expect(document.activeElement).toBe(opener);
    opener.remove();
  });
});
