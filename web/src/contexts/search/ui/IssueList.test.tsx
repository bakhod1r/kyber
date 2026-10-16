import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { IssueList } from "./IssueList";
import { type QuickFilterState, QuickFilters } from "./QuickFilters";
import { useState } from "react";

const hit = (key: string, title: string) => ({
  key, project_key: "KYB", title, type: "task", status: "todo", status_name: "To Do", status_category: "todo",
  priority: "medium", assignee_id: null, resolution: "", points: 3,
});
const q = (jql: string) => `GET /api/v1/search?jql=${encodeURIComponent(jql)}`;

describe("IssueList and quick filters", () => {
  it("lists, sorts and filters the project's issues", async () => {
    const onOpen = vi.fn();
    const calls = mockApi({
      [q('project = "KYB" ORDER BY key')]: { status: 200, body: { total: 2, items: [hit("KYB-1", "One"), hit("KYB-2", "Two")] } },
      [q('project = "KYB" ORDER BY key DESC')]: { status: 200, body: { total: 2, items: [hit("KYB-2", "Two"), hit("KYB-1", "One")] } },
      [q('project = "KYB" AND (assignee = currentUser()) ORDER BY key DESC')]: { status: 200, body: { total: 1, items: [hit("KYB-2", "Two")] } },
      [q('project = "KYB" AND (assignee = currentUser())')]: { status: 200, body: { total: 1, items: [hit("KYB-2", "Two")] } },
      "GET /api/v1/filters": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<IssueList projectKey="KYB" onOpen={onOpen} />);
    const list = await screen.findByRole("region", { name: "Issue list" });
    expect(await within(list).findByText("2 issues")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: /^Key/ }));
    await waitFor(() => expect(within(list).getAllByRole("row")[1]).toHaveTextContent("KYB-2"));
    await userEvent.click(within(list).getByRole("button", { name: "Only my issues" }));
    expect(await within(list).findByText("1 issue")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: "KYB-2" }));
    expect(onOpen).toHaveBeenCalledWith("KYB-2");
    expect(calls.some((c) => c.key.includes("currentUser"))).toBe(true);
  });

  it("reports the keys a saved filter matches, and clears", async () => {
    mockApi({
      "GET /api/v1/filters": { status: 200, body: { items: [{ id: "f", owner_id: "u", name: "Bugs", jql: "type = bug ORDER BY key", shared: false, version: 1, mine: true }] } },
      [q('project = "KYB" AND (type = bug)')]: { status: 200, body: { total: 1, items: [hit("KYB-7", "Bug")] } },
    });
    const onKeys = vi.fn();
    function Host() {
      const [value, setValue] = useState<QuickFilterState>({ quick: [], saved: "" });
      return <QuickFilters projectKey="KYB" value={value} onChange={setValue} onKeys={onKeys} />;
    }
    renderWithProviders(<Host />);
    await userEvent.selectOptions(await screen.findByLabelText("Saved filter"), "Bugs");
    await waitFor(() => expect(onKeys).toHaveBeenLastCalledWith(new Set(["KYB-7"])));
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    await waitFor(() => expect(onKeys).toHaveBeenLastCalledWith(null));
  });

  it("selects issues for a bulk change and offers the bulk panel (G9)", async () => {
    mockApi({
      [q('project = "KYB" ORDER BY key')]: { status: 200, body: { total: 2, items: [hit("KYB-1", "One"), hit("KYB-2", "Two")] } },
      "GET /api/v1/filters": { status: 200, body: { items: [] } },
    });
    const bulk = vi.fn((keys: string[], done: () => void) => (
      <button type="button" onClick={done}>
        bulk {keys.join(",")}
      </button>
    ));
    renderWithProviders(<IssueList projectKey="KYB" bulk={bulk} />);
    await userEvent.click(await screen.findByRole("checkbox", { name: "Select KYB-2" }));
    expect(screen.getByRole("button", { name: "bulk KYB-2" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("checkbox", { name: "Select all" }));
    await userEvent.click(screen.getByRole("button", { name: "bulk KYB-1,KYB-2" }));
    expect(screen.getByRole("checkbox", { name: "Select KYB-1" })).not.toBeChecked(); // done clears the selection
    expect(screen.queryByRole("button", { name: /^bulk/ })).not.toBeInTheDocument();
  });
});
