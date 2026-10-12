import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Issue } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { IssueLinks } from "./IssueLinks";

const other = (n: number, title: string): Issue => ({
  id: `i-${n}`, key: `KYB-${n}`, title, type: "task", status: "todo", description: "", priority: "medium",
  assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 1,
});

describe("IssueLinks (KYB-S45)", () => {
  it("lists links from this issue's side, adds and removes them", async () => {
    let items = [{ id: "l-1", type: "blocks", phrase: "is blocked by", issue: other(1, "API") }];
    const calls = mockApi({
      "GET /api/v1/issues/KYB-2/links": () => ({ status: 200, body: { items } }),
      "POST /api/v1/issues/KYB-2/links": () => {
        items = [...items, { id: "l-2", type: "relates", phrase: "relates to", issue: other(3, "Docs") }];
        return { status: 201, body: items[1] };
      },
      "DELETE /api/v1/links/l-1": () => {
        items = items.filter((l) => l.id !== "l-1");
        return { status: 204 };
      },
    });
    const onOpen = vi.fn();
    renderWithProviders(<IssueLinks issueKey="KYB-2" onOpen={onOpen} />);
    const list = await screen.findByRole("list", { name: "Linked issues" });
    expect(await within(list).findByText("is blocked by")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: /KYB-1 API/ }));
    expect(onOpen).toHaveBeenCalledWith("KYB-1");

    await userEvent.selectOptions(screen.getByLabelText("Link type"), "relates");
    await userEvent.type(screen.getByLabelText("Issue key to link"), "kyb-3");
    await userEvent.click(screen.getByRole("button", { name: "Link" }));
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ type: "relates", to_key: "KYB-3" }));
    expect(await within(list).findByText("relates to")).toBeInTheDocument();

    await userEvent.click(within(list).getByRole("button", { name: "Remove link to KYB-1" }));
    await waitFor(() => expect(within(list).queryByText("is blocked by")).not.toBeInTheDocument());
  });

  it("shows a refused link", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-2/links": { status: 200, body: { items: [] } },
      "POST /api/v1/issues/KYB-2/links": { status: 409, body: { type: "about:blank", title: "t", status: 409, code: "ISSUE_LINK_EXISTS", detail: "these issues are already linked this way" } },
    });
    renderWithProviders(<IssueLinks issueKey="KYB-2" />);
    expect(await screen.findByText("No linked issues.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Issue key to link"), "KYB-1");
    await userEvent.click(screen.getByRole("button", { name: "Link" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("already linked");
  });
});
