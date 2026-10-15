import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { Automation } from "./Automation";

const workflow = { initial: "todo", statuses: [{ id: "todo", name: "To Do", category: "todo" }, { id: "in_review", name: "In Review", category: "in_progress" }], transitions: [] };
const rule = {
  id: "r-1", project_key: "KYB", owner_id: "u-1", name: "Review bugs", trigger: "issue.transitioned", to_status: "in_review",
  condition: "type = bug", actions: [{ kind: "assign", value: "reporter" }], enabled: true, version: 1,
};

describe("Automation (G8)", () => {
  it("creates a rule with a trigger, a condition and two actions", async () => {
    let rules: (typeof rule)[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/automation/rules": () => ({ status: 200, body: { items: rules } }),
      "GET /api/v1/projects/KYB/automation/runs": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "POST /api/v1/projects/KYB/automation/rules": () => {
        rules = [rule];
        return { status: 201, body: rule };
      },
    });
    renderWithProviders(<Automation projectKey="KYB" />);
    expect(await screen.findByText("No rules yet.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Rule name"), "Review bugs");
    await userEvent.selectOptions(screen.getByLabelText("When"), "issue.transitioned");
    await userEvent.selectOptions(screen.getByLabelText("To status"), "in_review");
    await userEvent.type(screen.getByLabelText("If the issue matches (JQL, optional)"), "type = bug");
    await userEvent.selectOptions(screen.getByLabelText("Action 1"), "assign");
    await userEvent.type(screen.getByLabelText("Value of action 1"), "reporter");
    await userEvent.click(screen.getByRole("button", { name: "Add action" }));
    await userEvent.selectOptions(screen.getByLabelText("Action 2"), "comment");
    await userEvent.type(screen.getByLabelText("Value of action 2"), "On it");
    await userEvent.click(screen.getByRole("button", { name: "Create rule" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({
        name: "Review bugs", trigger: "issue.transitioned", to_status: "in_review", condition: "type = bug",
        actions: [{ kind: "assign", value: "reporter" }, { kind: "comment", value: "On it" }],
      }),
    );
    expect(await screen.findByRole("listitem", { name: "Review bugs" })).toBeInTheDocument();
  });

  it("turns a rule off, deletes it and shows recent runs", async () => {
    let rules = [rule];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/automation/rules": () => ({ status: 200, body: { items: rules } }),
      "GET /api/v1/projects/KYB/automation/runs": {
        status: 200, body: { items: [{ rule_id: "r-1", issue_key: "KYB-3", at: "2026-10-15T09:00:00Z", ok: false, detail: "assign: not allowed" }] },
      },
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "PUT /api/v1/automation/rules/r-1": (init) => {
        rules = [{ ...rule, enabled: JSON.parse(String(init?.body)).enabled, version: 2 }];
        return { status: 200, body: rules[0] };
      },
      "DELETE /api/v1/automation/rules/r-1": () => {
        rules = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<Automation projectKey="KYB" />);
    const item = await screen.findByRole("listitem", { name: "Review bugs" });
    await userEvent.click(within(item).getByRole("checkbox", { name: "Enabled" }));
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("PUT"))?.body).toMatchObject({ enabled: false, version: 1 }));
    expect(screen.getByRole("listitem", { name: /KYB-3/ })).toHaveTextContent("assign: not allowed");
    await userEvent.click(within(await screen.findByRole("listitem", { name: "Review bugs" })).getByRole("button", { name: "Delete rule Review bugs" }));
    expect(await screen.findByText("No rules yet.")).toBeInTheDocument();
  });
});
