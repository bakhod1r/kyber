import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { type Issue, parseOptions } from "../application";
import { CustomFieldsAdmin } from "./CustomFieldsAdmin";
import { IssueDetails } from "./IssueDetails";

const tier = { id: "f-1", project_key: "KYB", name: "Tier", type: "select", options: ["Gold", "Silver"], version: 1 };
const due = { id: "f-2", project_key: "KYB", name: "Due", type: "date", options: [], version: 1 };
const notes = { id: "f-3", project_key: "KYB", name: "Budget", type: "number", options: [], version: 1 };
const forbidden = { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "x" };

describe("custom fields", () => {
  it("parses options", () => {
    expect(parseOptions(" Gold, Silver\nBronze ,, ")).toEqual(["Gold", "Silver", "Bronze"]);
  });

  it("admins add, rename and delete fields", async () => {
    let items: (typeof tier)[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/fields": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/fields": () => {
        items = [tier];
        return { status: 201, body: tier };
      },
      "PUT /api/v1/fields/f-1": () => {
        items = [{ ...tier, name: "Level", version: 2 }];
        return { status: 200, body: items[0] };
      },
      "DELETE /api/v1/fields/f-1": () => {
        items = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<CustomFieldsAdmin projectKey="KYB" />);
    expect(await screen.findByText("No custom fields yet.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Field name"), "Tier");
    await userEvent.selectOptions(screen.getByLabelText("Field type"), "select");
    await userEvent.type(screen.getByLabelText("Options"), "Gold, Silver");
    await userEvent.click(screen.getByRole("button", { name: "Add field" }));
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/fields")?.body).toEqual({ name: "Tier", type: "select", options: ["Gold", "Silver"] });
    const nameBox = await screen.findByLabelText("Name of Tier");
    await userEvent.clear(nameBox);
    await userEvent.type(nameBox, "Level");
    await userEvent.click(screen.getByRole("button", { name: "Save Tier" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PUT /api/v1/fields/f-1")?.body).toEqual({ version: 1, name: "Level", options: ["Gold", "Silver"] }));
    await userEvent.click(await screen.findByRole("button", { name: "Delete Level" }));
    expect(await screen.findByText("No custom fields yet.")).toBeInTheDocument();
  });

  it("explains that only admins change fields", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/fields": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/fields": { status: 403, body: forbidden },
    });
    renderWithProviders(<CustomFieldsAdmin projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Field name"), "Notes");
    await userEvent.click(screen.getByRole("button", { name: "Add field" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Only project admins can change fields.");
  });

  it("fills in an issue's details by field type", async () => {
    const issue: Issue = {
      id: "i-1", key: "KYB-1", title: "Deal", type: "task", status: "todo", description: "", priority: "medium",
      assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 3, custom_fields: { "f-1": "Gold" },
    };
    const calls = mockApi({
      "GET /api/v1/projects/KYB/fields": { status: 200, body: { items: [tier, due, notes] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: { ...issue, version: 4 } },
    });
    renderWithProviders(<IssueDetails issue={issue} projectKey="KYB" />);
    expect(await screen.findByLabelText("Tier")).toHaveValue("Gold");
    await userEvent.selectOptions(screen.getByLabelText("Tier"), "Silver");
    await userEvent.type(screen.getByLabelText("Due"), "2026-12-01");
    await userEvent.click(screen.getByRole("button", { name: "Save details" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({ version: 3, custom_fields: { "f-1": "Silver", "f-2": "2026-12-01" } }),
    );
    expect(await screen.findByRole("status")).toHaveTextContent("Details saved");
  });

  it("shows nothing without fields, and refusals", async () => {
    const issue = { id: "i", key: "KYB-1", title: "x", type: "task", status: "todo", description: "", priority: "medium", assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: "a", version: 1 } as Issue;
    mockApi({
      "GET /api/v1/projects/KYB/fields": { status: 200, body: { items: [notes] } },
      "PATCH /api/v1/issues/KYB-1": { status: 422, body: { type: "about:blank", title: "t", status: 422, code: "VALIDATION_FAILED", detail: "not a valid number" } },
    });
    renderWithProviders(<IssueDetails issue={issue} projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Budget"), "12");
    await userEvent.click(screen.getByRole("button", { name: "Save details" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("not a valid number");
  });
});
