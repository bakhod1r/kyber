import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AppRoutes } from "./App";
import { mockApi, renderWithProviders } from "../shared/test/test-utils";

const api = () =>
  mockApi({
    "GET /api/v1/me": { status: 200, body: { id: "u-1", email: "ali@x.uz", name: "Ali" } },
    "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p-1", key: "KYB", name: "Kyber" }] } },
    "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
    "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-1", email: "ali@x.uz", name: "Ali", role: "admin" }] } },
  });

describe("ProjectPage navigation", () => {
  it("a view shows one row of project tabs with Settings at its end, and no admin forms", async () => {
    api();
    renderWithProviders(<AppRoutes />, "/projects/KYB");
    const nav = await screen.findByRole("navigation", { name: "Project views" });
    expect(within(nav).getByRole("link", { name: "Settings" })).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Project settings" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Invite by email")).not.toBeInTheDocument();
  });

  it("settings pages mark Settings and list their sections, Members first", async () => {
    api();
    renderWithProviders(<AppRoutes />, "/projects/KYB/workflow");
    const views = await screen.findByRole("navigation", { name: "Project views" });
    expect(within(views).getByRole("link", { name: "Settings" })).toHaveAttribute("aria-current", "page");
    const settings = screen.getByRole("navigation", { name: "Project settings" });
    const links = within(settings).getAllByRole("link").map((l) => l.textContent);
    expect(links[0]).toBe("Members");
    expect(links).toEqual(expect.arrayContaining(["Workflow", "Board settings", "Fields", "Permissions", "Audit"]));
  });

  it("the Members settings page invites people", async () => {
    api();
    renderWithProviders(<AppRoutes />, "/projects/KYB/members");
    expect(await screen.findByLabelText("Invite by email")).toBeInTheDocument();
  });
});
