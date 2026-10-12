import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { PermissionGrants } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { PermissionScheme } from "./PermissionScheme";

const admin = { kind: "role", value: "admin" } as const;
const member = { kind: "role", value: "member" } as const;
const defaults = (): PermissionGrants[] => [
  { permission: "BROWSE_PROJECTS", grants: [admin, member, { kind: "role", value: "viewer" }] },
  { permission: "CREATE_ISSUES", grants: [member, admin] },
  { permission: "ADMINISTER_PROJECTS", grants: [admin] },
];
const members = { items: [{ user_id: "u-bob", email: "bob@x.uz", name: "Bob", role: "member" }] };

describe("PermissionScheme", () => {
  it("edits role and user holders, saves and resets", async () => {
    let scheme = { custom: false, permissions: defaults() };
    const calls = mockApi({
      "GET /api/v1/projects/KYB/permissions": () => ({ status: 200, body: scheme }),
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "PUT /api/v1/projects/KYB/permissions": (init) => {
        scheme = { custom: true, permissions: (JSON.parse(String(init?.body)) as { permissions: PermissionGrants[] }).permissions };
        return { status: 200, body: scheme };
      },
      "DELETE /api/v1/projects/KYB/permissions": () => {
        scheme = { custom: false, permissions: defaults() };
        return { status: 204 };
      },
    });
    renderWithProviders(<PermissionScheme projectKey="KYB" />);
    expect(await screen.findByText(/uses the default permission scheme/)).toBeInTheDocument();
    expect(screen.getByLabelText("Browse projects: Admin")).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Reset to default" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByLabelText("Create issues: Viewer"));
    await userEvent.click(screen.getByLabelText("Create issues: Member"));
    await userEvent.selectOptions(screen.getByLabelText("Add a user to Create issues"), "u-bob");
    await userEvent.click(screen.getByRole("button", { name: "Remove Bob from Create issues" }));
    await userEvent.selectOptions(screen.getByLabelText("Add a user to Create issues"), "u-bob");
    await userEvent.click(screen.getByRole("button", { name: "Save permissions" }));

    await waitFor(() => expect(calls.some((c) => c.key === "PUT /api/v1/projects/KYB/permissions")).toBe(true));
    const put = calls.find((c) => c.key === "PUT /api/v1/projects/KYB/permissions")?.body as { permissions: PermissionGrants[] };
    expect(put.permissions[1]).toEqual({
      permission: "CREATE_ISSUES",
      grants: [admin, { kind: "role", value: "viewer" }, { kind: "user", value: "u-bob" }],
    });
    expect(await screen.findByText(/has its own permission scheme/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Reset to default" }));
    expect(await screen.findByText(/uses the default permission scheme/)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Reset to the default scheme");
  });

  it("shows API errors and explains when only admins may edit", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/permissions": { status: 200, body: { custom: false, permissions: defaults() } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
      "PUT /api/v1/projects/KYB/permissions": {
        status: 422,
        body: { type: "about:blank", title: "Some fields are invalid.", status: 422, code: "VALIDATION_FAILED", detail: "the admin role must keep BROWSE_PROJECTS" },
      },
    });
    renderWithProviders(<PermissionScheme projectKey="KYB" />);
    await userEvent.click(await screen.findByRole("button", { name: "Save permissions" }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });

  it("tells non-admins they cannot change permissions", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/permissions": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "x" } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: members },
    });
    renderWithProviders(<PermissionScheme projectKey="KYB" />);
    expect(await screen.findByText(/Only project admins can change permissions/)).toBeInTheDocument();
  });
});
