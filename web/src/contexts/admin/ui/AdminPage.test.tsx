import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { AdminPage } from "./AdminPage";

const members = { items: [{ id: "u-1", name: "Ann", email: "ann@x.uz", role: "owner" }, { id: "u-2", name: "Bob", email: "bob@x.uz", role: "member" }] };
const problem = (status: number, detail: string) => ({ status, body: { type: "about:blank", title: "t", status, code: "X", detail } });

describe("admin page", () => {
  it("changes roles, lists projects and retries stuck events", async () => {
    const calls = mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: members },
      "PUT /api/v1/admin/members/u-2": { status: 204 },
      "PUT /api/v1/admin/members/u-1": problem(409, "A workspace must keep at least one owner."),
      "GET /api/v1/admin/projects": { status: 200, body: { items: [{ key: "KYB", name: "Kyber", members: 1 }] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [{ id: 7, event: "issue.created", actor_id: "u-1", at: "2026-10-13T09:00:00Z", attempts: 10, last_error: "boom" }] } },
      "POST /api/v1/admin/dead-letters/7/retry": { status: 204 },
    });
    const user = userEvent.setup();
    renderWithProviders(<AdminPage />);
    await user.selectOptions(await screen.findByLabelText("Role of Bob"), "admin");
    await waitFor(() => expect(calls.find((c) => c.key === "PUT /api/v1/admin/members/u-2")?.body).toEqual({ role: "admin" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Role changed");
    await user.selectOptions(screen.getByLabelText("Role of Ann"), "member");
    expect(await screen.findByRole("alert")).toHaveTextContent("at least one owner");

    expect(within(screen.getByRole("region", { name: "Projects" })).getByText(/1 member$/)).toBeInTheDocument();
    const stuck = screen.getByRole("region", { name: "Stuck events" });
    expect(within(stuck).getByText("boom")).toBeInTheDocument();
    await user.click(within(stuck).getByRole("button", { name: "Retry #7" }));
    expect(calls.some((c) => c.key === "POST /api/v1/admin/dead-letters/7/retry")).toBe(true);
    expect(await screen.findByRole("status")).toHaveTextContent("Queued again");
  });

  it("shows empty lists and refuses non-administrators", async () => {
    mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AdminPage />);
    expect(await screen.findByText("Nothing is stuck.")).toBeInTheDocument();
    expect(screen.getByText("No projects yet.")).toBeInTheDocument();
  });

  it("explains a refusal", async () => {
    const calls = mockApi({ "GET /api/v1/admin/access": { status: 200, body: { admin: false } } });
    renderWithProviders(<AdminPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Only workspace administrators");
    expect(calls.some((c) => c.key === "GET /api/v1/admin/members")).toBe(false);
  });

  it("shows a failure", async () => {
    mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": problem(500, "db down"),
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AdminPage />);
    expect(await screen.findByRole("alert")).toHaveTextContent("db down");
  });

  it("maps SSO groups to roles for site administrators", async () => {
    const mapping = { id: "m-1", group: "devs", scope: "project", workspace_id: "w-1", project_key: "KYB", role: "member", created_at: "2026-10-15T09:00:00Z" };
    const calls = mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/sso/group-mappings": { status: 200, body: { items: [mapping] } },
      "POST /api/v1/admin/sso/group-mappings": { status: 201, body: mapping },
      "DELETE /api/v1/admin/sso/group-mappings/m-1": { status: 204 },
    });
    const user = userEvent.setup();
    renderWithProviders(<AdminPage />);
    const sso = await screen.findByRole("region", { name: "SSO group mappings" });
    expect(await within(sso).findByText("devs")).toBeInTheDocument();
    await user.type(within(sso).getByLabelText("Group"), "admins");
    await user.selectOptions(within(sso).getByLabelText("Scope"), "project");
    await user.type(within(sso).getByLabelText("Project key"), "KYB");
    await user.selectOptions(within(sso).getByLabelText("Mapped role"), "admin");
    await user.click(within(sso).getByRole("button", { name: "Add mapping" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/admin/sso/group-mappings")?.body).toEqual({ group: "admins", scope: "project", project_key: "KYB", role: "admin" }));
    await user.click(within(sso).getByRole("button", { name: "Remove mapping devs" }));
    await waitFor(() => expect(calls.some((c) => c.key === "DELETE /api/v1/admin/sso/group-mappings/m-1")).toBe(true));
  });

  it("hides SSO group mappings from workspace administrators", async () => {
    mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/sso/group-mappings": problem(403, "site admins only"),
    });
    renderWithProviders(<AdminPage />);
    expect(await screen.findByText("Nothing is stuck.")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("region", { name: "SSO group mappings" })).not.toBeInTheDocument());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("adds, disables and removes SSO connections without ever showing a secret", async () => {
    const conn = { id: "c-1", kind: "oidc", name: "Acme Okta", issuer: "https://acme.okta.com", client_id: "kyber", groups_claim: "", enabled: true, created_at: "2026-10-15T09:00:00Z", domains: [] };
    const calls = mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/sso/connections": { status: 200, body: { items: [conn] } },
      "POST /api/v1/admin/sso/connections": { status: 201, body: conn },
      "PATCH /api/v1/admin/sso/connections/c-1": { status: 200, body: { ...conn, enabled: false } },
      "DELETE /api/v1/admin/sso/connections/c-1": { status: 204 },
    });
    const user = userEvent.setup();
    renderWithProviders(<AdminPage />);
    const sso = await screen.findByRole("region", { name: "SSO connections" });
    expect(await within(sso).findByText("Acme Okta")).toBeInTheDocument();
    expect(within(sso).getByText(/\/api\/v1\/auth\/sso\/c-1\/callback/)).toBeInTheDocument();
    await user.type(within(sso).getByLabelText("Button label"), "Entra");
    await user.type(within(sso).getByLabelText("Issuer URL"), "https://login.example.com");
    await user.type(within(sso).getByLabelText("Client ID"), "kid");
    await user.type(within(sso).getByLabelText("Client secret"), "shh");
    await user.click(within(sso).getByRole("button", { name: "Add connection" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "POST /api/v1/admin/sso/connections")?.body).toEqual({
        name: "Entra",
        issuer: "https://login.example.com",
        client_id: "kid",
        client_secret: "shh",
        groups_claim: "",
      }),
    );
    expect(within(sso).getByLabelText("Client secret")).toHaveValue("");
    await user.click(within(sso).getByRole("button", { name: "Disable Acme Okta" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/admin/sso/connections/c-1")?.body).toEqual({ enabled: false }));
    await user.click(within(sso).getByRole("button", { name: "Remove connection Acme Okta" }));
    await waitFor(() => expect(calls.some((c) => c.key === "DELETE /api/v1/admin/sso/connections/c-1")).toBe(true));
  });

  it("explains that SSO connections need an encryption key", async () => {
    mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/sso/connections": { status: 200, body: { items: [] } },
      "POST /api/v1/admin/sso/connections": problem(409, "Set KYBER_ENCRYPTION_KEY to store SSO connections."),
    });
    const user = userEvent.setup();
    renderWithProviders(<AdminPage />);
    const sso = await screen.findByRole("region", { name: "SSO connections" });
    await user.type(within(sso).getByLabelText("Button label"), "X");
    await user.type(within(sso).getByLabelText("Issuer URL"), "https://x.example");
    await user.type(within(sso).getByLabelText("Client ID"), "a");
    await user.type(within(sso).getByLabelText("Client secret"), "b");
    await user.click(within(sso).getByRole("button", { name: "Add connection" }));
    expect(await within(sso).findByRole("alert")).toHaveTextContent("KYBER_ENCRYPTION_KEY");
  });

  it("adds and verifies the email domains of an SSO connection", async () => {
    const pending = { name: "acme.uz", verified: false, txt: "kyber-verify=abc123" };
    const conn = { id: "c-1", kind: "oidc", name: "Acme Okta", issuer: "https://acme.okta.com", client_id: "kyber", groups_claim: "", enabled: true, created_at: "2026-10-15T09:00:00Z", domains: [pending] };
    const calls = mockApi({
      "GET /api/v1/admin/access": { status: 200, body: { admin: true } },
      "GET /api/v1/admin/members": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/projects": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/dead-letters": { status: 200, body: { items: [] } },
      "GET /api/v1/admin/sso/connections": { status: 200, body: { items: [conn] } },
      "POST /api/v1/admin/sso/connections/c-1/domains": { status: 200, body: conn },
      "POST /api/v1/admin/sso/connections/c-1/domains/acme.uz/verify": problem(409, "The TXT record kyber-verify=<token> was not found on the domain yet."),
      "DELETE /api/v1/admin/sso/connections/c-1/domains/acme.uz": { status: 200, body: { ...conn, domains: [] } },
    });
    const user = userEvent.setup();
    renderWithProviders(<AdminPage />);
    const sso = await screen.findByRole("region", { name: "SSO connections" });
    expect(await within(sso).findByText("kyber-verify=abc123")).toBeInTheDocument();
    expect(within(sso).getByText("acme.uz")).toBeInTheDocument();
    await user.click(within(sso).getByRole("button", { name: "Verify acme.uz" }));
    expect(await within(sso).findByRole("alert")).toHaveTextContent("not found on the domain yet");
    await user.type(within(sso).getByLabelText("Email domain for Acme Okta"), "corp.uz");
    await user.click(within(sso).getByLabelText("Trusted (skip DNS check) for Acme Okta"));
    await user.click(within(sso).getByRole("button", { name: "Add domain to Acme Okta" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/admin/sso/connections/c-1/domains")?.body).toEqual({ domain: "corp.uz", trusted: true }));
    await user.click(within(sso).getByRole("button", { name: "Remove domain acme.uz" }));
    await waitFor(() => expect(calls.some((c) => c.key === "DELETE /api/v1/admin/sso/connections/c-1/domains/acme.uz")).toBe(true));
  });
});
