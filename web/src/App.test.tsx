import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { AppRoutes } from "./App";
import { mockApi, renderWithProviders } from "./test-utils";

const me = { id: "u-1", email: "ali@x.uz", name: "Ali" };

describe("App routing and auth (KYB-S14 AC1, AC2)", () => {
  it("redirects anonymous users to the login page", async () => {
    mockApi({ "GET /api/v1/me": { status: 401, body: { error: "authentication required" } } });
    renderWithProviders(<AppRoutes />, "/");
    expect(await screen.findByRole("heading", { name: "Log in to Kyber" })).toBeInTheDocument();
  });

  it("logs in and shows the user's projects", async () => {
    let loggedIn = false;
    const calls = mockApi({
      "GET /api/v1/me": () => (loggedIn ? { status: 200, body: me } : { status: 401, body: { error: "authentication required" } }),
      "POST /api/v1/auth/login": () => {
        loggedIn = true;
        return { status: 200, body: { token: "t" } };
      },
      "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p-1", key: "KYB", name: "Kyber" }] } },
    });
    renderWithProviders(<AppRoutes />, "/login");
    await userEvent.type(await screen.findByLabelText("Email"), "ali@x.uz");
    await userEvent.type(screen.getByLabelText("Password"), "long enough pw");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));

    expect(await screen.findByRole("link", { name: "KYB Kyber" })).toHaveAttribute("href", "/projects/KYB");
    expect(screen.getByText("Ali")).toBeInTheDocument();
    expect(calls.find((c) => c.key === "POST /api/v1/auth/login")?.body).toEqual({ email: "ali@x.uz", password: "long enough pw" });
  });

  it("regression: logging in after being redirected from / lands on projects", async () => {
    let loggedIn = false;
    mockApi({
      "GET /api/v1/me": () => (loggedIn ? { status: 200, body: me } : { status: 401, body: { error: "authentication required" } }),
      "POST /api/v1/auth/login": () => {
        loggedIn = true;
        return { status: 200, body: { token: "t" } };
      },
      "GET /api/v1/projects": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AppRoutes />, "/"); // caches "anonymous", redirects to /login
    await userEvent.type(await screen.findByLabelText("Email"), "ali@x.uz");
    await userEvent.type(screen.getByLabelText("Password"), "long enough pw");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));
    expect(await screen.findByText(/No projects yet/)).toBeInTheDocument();
  });

  it("shows login errors from the server", async () => {
    mockApi({
      "GET /api/v1/me": { status: 401, body: { error: "authentication required" } },
      "POST /api/v1/auth/login": { status: 401, body: { error: "invalid email or password" } },
    });
    renderWithProviders(<AppRoutes />, "/login");
    await userEvent.type(await screen.findByLabelText("Email"), "ali@x.uz");
    await userEvent.type(screen.getByLabelText("Password"), "wrong password");
    await userEvent.click(screen.getByRole("button", { name: "Log in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("invalid email or password");
  });

  it("signs up, then logs in automatically", async () => {
    let loggedIn = false;
    const calls = mockApi({
      "GET /api/v1/me": () => (loggedIn ? { status: 200, body: me } : { status: 401, body: { error: "authentication required" } }),
      "POST /api/v1/auth/signup": { status: 201, body: me },
      "POST /api/v1/auth/login": () => {
        loggedIn = true;
        return { status: 200, body: { token: "t" } };
      },
      "GET /api/v1/projects": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AppRoutes />, "/signup");
    await userEvent.type(await screen.findByLabelText("Name"), "Ali");
    await userEvent.type(screen.getByLabelText("Email"), "ali@x.uz");
    await userEvent.type(screen.getByLabelText("Password"), "long enough pw");
    await userEvent.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByText(/No projects yet/)).toBeInTheDocument();
    expect(calls.map((c) => c.key)).toContain("POST /api/v1/auth/signup");
  });

  it("creates a project and opens its board", async () => {
    const projects: { id: string; key: string; name: string }[] = [];
    mockApi({
      "GET /api/v1/me": { status: 200, body: me },
      "GET /api/v1/projects": () => ({ status: 200, body: { items: projects } }),
      "POST /api/v1/projects": () => {
        projects.push({ id: "p-1", key: "OPS", name: "Operations" });
        return { status: 201, body: projects[0] };
      },
      "GET /api/v1/projects/OPS/issues": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/OPS/members": { status: 200, body: { items: [{ user_id: "u-1", email: "ali@x.uz", name: "Ali", role: "admin" }] } },
    });
    renderWithProviders(<AppRoutes />, "/");
    await userEvent.type(await screen.findByLabelText("Key"), "OPS");
    await userEvent.type(screen.getByLabelText("Project name"), "Operations");
    await userEvent.click(screen.getByRole("button", { name: "Create project" }));
    await waitFor(() => expect(screen.getByRole("heading", { name: /Operations/ })).toBeInTheDocument());
    expect(await screen.findByRole("region", { name: "To Do" })).toBeInTheDocument();
  });

  it("admins can invite members", async () => {
    const calls = mockApi({
      "GET /api/v1/me": { status: 200, body: me },
      "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p-1", key: "KYB", name: "Kyber" }] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-1", email: "ali@x.uz", name: "Ali", role: "admin" }] } },
      "POST /api/v1/projects/KYB/members": { status: 204 },
    });
    renderWithProviders(<AppRoutes />, "/projects/KYB");
    await userEvent.type(await screen.findByLabelText("Invite by email"), "bob@x.uz");
    await userEvent.selectOptions(screen.getByLabelText("Role"), "viewer");
    await userEvent.click(screen.getByRole("button", { name: "Invite" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/members")?.body).toEqual({ email: "bob@x.uz", role: "viewer" }),
    );
  });
});
