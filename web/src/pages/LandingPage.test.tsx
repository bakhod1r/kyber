import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AppRoutes } from "../App";
import { mockApi, renderWithProviders } from "../test-utils";

const unauthorized = { status: 401, body: { type: "about:blank", title: "Please log in.", status: 401, code: "AUTH_REQUIRED" } };

describe("Landing page (KYB-S36)", () => {
  it("AC1: visitors see what Kyber is and how to start", async () => {
    mockApi({ "GET /api/v1/me": unauthorized });
    renderWithProviders(<AppRoutes />, "/");
    expect(await screen.findByRole("heading", { level: 1, name: /plan, track and ship/i })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Start free" })[0]).toHaveAttribute("href", "/signup");
    expect(screen.getByRole("link", { name: "Log in" })).toHaveAttribute("href", "/login");
    const features = screen.getByRole("region", { name: "Features" });
    for (const f of ["Boards", "Backlog & sprints", "Reports", "Permissions", "Import from Jira", "Self-hosted"]) {
      expect(features).toHaveTextContent(f);
    }
    expect(screen.getByRole("link", { name: /GitHub/ })).toHaveAttribute("href", "https://github.com/bakhod1r/kyber");
  });

  it("AC2: signed-in users go straight to their projects", async () => {
    mockApi({
      "GET /api/v1/me": { status: 200, body: { id: "u-1", email: "a@x.uz", name: "Ali" } },
      "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p-1", key: "KYB", name: "Kyber" }] } },
    });
    renderWithProviders(<AppRoutes />, "/");
    expect(await screen.findByText("Kyber")).toBeInTheDocument();
    expect(screen.queryByRole("heading", { level: 1, name: /plan, track and ship/i })).not.toBeInTheDocument();
  });
});
