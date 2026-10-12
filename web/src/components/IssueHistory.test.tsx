import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { IssueHistory } from "./IssueHistory";

const entry = (field: string, from: string, to: string, extra = {}) => ({
  at: "2026-10-12T09:00:00Z", by_id: "u-ali", by_name: "Ali Valiyev", field, from, to, from_name: from, to_name: to, ...extra,
});

describe("IssueHistory", () => {
  it("lists every action with who, when and old → new values", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1/history": {
        status: 200,
        body: { items: [
          entry("created", "", "Login"),
          entry("status", "todo", "code_review"),
          entry("assignee", "u-1", "u-2", { from_name: "Bob", to_name: "Ali Valiyev" }),
          entry("estimate", "", "3"),
          entry("comment", "", "Ready for review"),
          entry("priority", "", "", { by_id: null, by_name: "" }),
        ] },
      },
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: { initial: "todo", statuses: [
        { id: "todo", name: "To Do", category: "todo" }, { id: "code_review", name: "Code Review", category: "in_progress" }], transitions: [] } },
    });
    renderWithProviders(<IssueHistory issueKey="KYB-1" projectKey="KYB" />);
    const list = await screen.findByRole("list", { name: "History" });
    const items = await within(list).findAllByRole("listitem");
    expect(items).toHaveLength(6);
    expect(items[0]).toHaveTextContent("Ali Valiyev created the issue");
    expect(items[1]).toHaveTextContent("Ali Valiyev changed Status: To Do → Code Review");
    expect(items[2]).toHaveTextContent("changed Assignee: Bob → Ali Valiyev");
    expect(items[3]).toHaveTextContent("changed Story points: None → 3");
    expect(items[4]).toHaveTextContent("Ali Valiyev commented: Ready for review");
    expect(items[5]).toHaveTextContent("System changed Priority");
    expect(within(items[0]!).getByText((_, el) => el?.tagName === "TIME")).toHaveAttribute("datetime", "2026-10-12T09:00:00Z");
  });
});
