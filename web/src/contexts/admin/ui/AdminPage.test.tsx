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
});
