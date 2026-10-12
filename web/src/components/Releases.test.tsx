import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { Releases } from "./Releases";

const v = (id: string, name: string, state: string, extra = {}) => ({
  id, project_key: "KYB", name, description: "", release_date: "2026-12-01T00:00:00Z", state, released_at: null, version: 1, issues: 3, done: 2, unresolved: 1, ...extra,
});

describe("Releases (S48)", () => {
  it("creates, releases moving unresolved work, archives and shows notes", async () => {
    let items = [v("v-1", "1.0", "unreleased"), v("v-2", "1.1", "unreleased", { issues: 0, done: 0, unresolved: 0 })];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/versions": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/versions": () => {
        items = [...items, v("v-3", "2.0", "unreleased")];
        return { status: 201, body: v("v-3", "2.0", "unreleased") };
      },
      "POST /api/v1/versions/v-1/release": () => {
        items = items.map((x) => (x.id === "v-1" ? { ...x, state: "released" } : x));
        return { status: 200, body: { version: { ...v("v-1", "1.0", "released") }, moved: 1 } };
      },
      "POST /api/v1/versions/v-2/archive": { status: 200, body: v("v-2", "1.1", "archived") },
      "DELETE /api/v1/versions/v-3": { status: 204 },
      "GET /api/v1/versions/v-1/release-notes": { status: 200, body: "# Release notes — 1.0" },
    });
    renderWithProviders(<Releases projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Version name"), "2.0");
    await userEvent.click(screen.getByRole("button", { name: "Create version" }));
    expect(await screen.findByRole("status")).toHaveTextContent("2.0 created");

    const row = screen.getByRole("row", { name: /^1\.0/ });
    await userEvent.selectOptions(within(row).getByLabelText("Unresolved to"), "v-2");
    await userEvent.click(within(row).getByRole("button", { name: "Release 1.0" }));
    expect(await screen.findByText("1.0 released; 1 unresolved moved on")).toBeInTheDocument();
    expect(calls.find((c) => c.key === "POST /api/v1/versions/v-1/release")?.body).toEqual({ move_to: "v-2" });

    await userEvent.click(within(screen.getByRole("row", { name: /^1\.1/ })).getByRole("button", { name: "Archive" }));
    expect(await screen.findByText("1.1 is archived")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Delete 2.0" }));
    expect(await screen.findByText("Version deleted")).toBeInTheDocument();

    await userEvent.click(within(screen.getByRole("row", { name: /^1\.0/ })).getByRole("button", { name: "Release notes" }));
    expect(await within(screen.getByRole("region", { name: "Release notes" })).findByText(/# Release notes/)).toBeInTheDocument();
  });

  it("explains an empty project and shows errors", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/versions": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/versions": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "admins only" } },
    });
    renderWithProviders(<Releases projectKey="KYB" />);
    expect(await screen.findByText(/No versions yet/)).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Version name"), "1.0");
    await userEvent.click(screen.getByRole("button", { name: "Create version" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("admins only");
  });
});
