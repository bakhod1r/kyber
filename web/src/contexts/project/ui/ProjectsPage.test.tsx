import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { ProjectsPage } from "./ProjectsPage";

describe("ProjectsPage (B10)", () => {
  it("creates a project from a template", async () => {
    const calls = mockApi({
      "GET /api/v1/projects": { status: 200, body: { items: [] } },
      "POST /api/v1/projects": { status: 201, body: { id: "p-1", key: "BUG", name: "Bugs", archived: false } },
    });
    renderWithProviders(<ProjectsPage />);
    await userEvent.type(await screen.findByLabelText("Key"), "bug");
    await userEvent.type(screen.getByLabelText("Project name"), "Bugs");
    await userEvent.selectOptions(screen.getByLabelText("Template"), "bug-tracking");
    await userEvent.click(screen.getByRole("button", { name: "Create project" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/projects")?.body).toEqual({ key: "BUG", name: "Bugs", template: "bug-tracking" }));
  });

  it("lists archived projects last, marked", async () => {
    mockApi({
      "GET /api/v1/projects": {
        status: 200,
        body: { items: [{ id: "1", key: "OLD", name: "Old", archived: true }, { id: "2", key: "NEW", name: "New", archived: false }] },
      },
    });
    renderWithProviders(<ProjectsPage />);
    const items = await screen.findAllByRole("listitem");
    expect(items[0]).toHaveTextContent("NEW");
    expect(items[1]).toHaveTextContent(/OLD\s*Old\s*Archived/);
  });
});
