import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { JiraImport } from "./ImportPage";

const csv = "Summary,Issue key\nLogin fails,PROJ-1\n";
const item = (issue_key: string | null) => ({
  line: 2, external_key: "PROJ-1", issue_key, title: "Login fails", type: "bug", status: "done", priority: "high",
  assignee_id: null, points: 3, created_at: "2026-03-01T09:00:00Z", resolved_at: null, warnings: ["assignee \"ghost\" is not a project member; left unassigned"],
});
const report = (dry: boolean) => ({
  dry_run: dry, items: [item(dry ? null : "KYB-1")], errors: [{ line: 3, external_key: "PROJ-3", message: "missing Summary" }], skipped: ["PROJ-0"],
});

async function pick(user: ReturnType<typeof userEvent.setup>) {
  const file = new File([csv], "jira.csv", { type: "text/csv" });
  await user.upload(screen.getByLabelText("Jira CSV export"), file);
}

describe("Jira import (KYB-S29)", () => {
  it("AC1: previews before importing, showing mapping, warnings and errors", async () => {
    const user = userEvent.setup();
    const calls = mockApi({ "POST /api/v1/projects/KYB/import/jira?dry_run=true": { status: 200, body: report(true) } });
    renderWithProviders(<JiraImport projectKey="KYB" />);
    expect(screen.getByRole("button", { name: "Preview" })).toBeDisabled();
    await pick(user);
    await user.click(screen.getByRole("button", { name: "Preview" }));
    const table = await screen.findByRole("table", { name: "Issues to import" });
    const row = within(table).getByText("PROJ-1").closest("tr") as HTMLElement;
    expect(row).toHaveTextContent("Login fails");
    expect(row).toHaveTextContent("Done");
    expect(row).toHaveTextContent("not a project member");
    expect(screen.getByRole("list", { name: "Rows with errors" })).toHaveTextContent("Line 3 · PROJ-3: missing Summary");
    expect(screen.getByText("1 already imported (skipped)")).toBeInTheDocument();
    expect(calls[0]?.body).toEqual({ csv });
  });

  it("AC2: imports after preview and links the new issues", async () => {
    const user = userEvent.setup();
    mockApi({
      "POST /api/v1/projects/KYB/import/jira?dry_run=true": { status: 200, body: report(true) },
      "POST /api/v1/projects/KYB/import/jira?dry_run=false": { status: 201, body: report(false) },
    });
    renderWithProviders(<JiraImport projectKey="KYB" />);
    await pick(user);
    await user.click(screen.getByRole("button", { name: "Preview" }));
    await user.click(await screen.findByRole("button", { name: "Import 1 issue" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Imported 1 issue.");
    expect(screen.getByRole("link", { name: "KYB-1" })).toHaveAttribute("href", "/projects/KYB?issue=KYB-1");
  });

  it("AC6: explains why the server refused", async () => {
    const user = userEvent.setup();
    mockApi({
      "POST /api/v1/projects/KYB/import/jira?dry_run=true": {
        status: 403, body: { type: "about:blank", title: "Forbidden", status: 403, code: "PROJECT_FORBIDDEN", detail: "only project admins can import" },
      },
    });
    renderWithProviders(<JiraImport projectKey="KYB" />);
    await pick(user);
    await user.click(screen.getByRole("button", { name: "Preview" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("only project admins can import");
  });

  it("S30: offers a CSV export of the project", () => {
    mockApi({});
    renderWithProviders(<JiraImport projectKey="KYB" />);
    const link = screen.getByRole("link", { name: "Export all issues (CSV)" });
    expect(link).toHaveAttribute("href", "/api/v1/projects/KYB/export.csv");
    expect(link).toHaveAttribute("download");
  });
});
