import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { AuditLog, describeAction } from "./AuditLog";

describe("AuditLog", () => {
  it("lists who did what and when, flagging refused attempts", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/audit": {
        status: 200,
        body: { items: [
          { at: "2026-10-12T09:05:00Z", actor_id: "u-2", actor_name: "Bob", action: "PUT /api/v1/projects/{key}/workflow", target: "/api/v1/projects/KYB/workflow", status: 403 },
          { at: "2026-10-12T09:00:00Z", actor_id: "u-1", actor_name: "Ali", action: "POST /api/v1/issues/{issueKey}/transitions", target: "/api/v1/issues/KYB-1/transitions", status: 200 },
        ] },
      },
    });
    renderWithProviders(<AuditLog projectKey="KYB" />);
    const rows = await screen.findAllByRole("row");
    expect(rows).toHaveLength(3); // header + 2
    expect(within(rows[1]!).getByText("Bob")).toBeInTheDocument();
    expect(rows[1]).toHaveTextContent("Changed the workflow");
    expect(rows[1]).toHaveTextContent("Refused (403)");
    expect(rows[2]).toHaveTextContent("Moved KYB-1");
    expect(rows[2]).toHaveTextContent("OK");
  });

  it("explains when the reader is not a project admin", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/audit": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "only project admins can read the audit log" } },
    });
    renderWithProviders(<AuditLog projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("only project admins");
  });

  it("names every API action in plain words", () => {
    const cases: [string, string, string][] = [
      ["POST /api/v1/projects/{key}/issues", "/api/v1/projects/KYB/issues", "Created an issue"],
      ["PATCH /api/v1/issues/{issueKey}", "/api/v1/issues/KYB-3", "Edited KYB-3"],
      ["POST /api/v1/issues/{issueKey}/comments", "/api/v1/issues/KYB-3/comments", "Commented on KYB-3"],
      ["POST /api/v1/issues/{issueKey}/rank", "/api/v1/issues/KYB-3/rank", "Ranked KYB-3"],
      ["POST /api/v1/projects/{key}/members", "/api/v1/projects/KYB/members", "Changed project members"],
      ["POST /api/v1/projects/{key}/boards", "/api/v1/projects/KYB/boards", "Created a board"],
      ["POST /api/v1/projects/{key}/import/jira", "/api/v1/projects/KYB/import/jira", "Imported from Jira"],
      ["POST /api/v1/sprints/{id}/complete", "/api/v1/sprints/s1/complete", "Completed a sprint"],
      ["DELETE /api/v1/boards/{id}", "/api/v1/boards/b1", "Deleted a board"],
      ["DELETE /api/v1/x/{id}", "/api/v1/x/1", "DELETE /api/v1/x/1"],
    ];
    for (const [action, target, want] of cases) expect(describeAction(action, target)).toBe(want);
  });
});
