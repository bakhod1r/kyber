import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { SearchPage } from "./SearchPage";

const hit = (key: string, title: string) => ({
  key, project_key: "KYB", title, type: "bug", status: "todo", status_name: "To Do", status_category: "todo",
  priority: "high", assignee_id: null, resolution: "", points: null,
});

describe("SearchPage", () => {
  it("runs JQL, shows results, saves and deletes a filter", async () => {
    let filters: { id: string; owner_id: string; name: string; jql: string; shared: boolean; version: number; mine: boolean }[] = [];
    const calls = mockApi({
      "GET /api/v1/search?jql=": { status: 200, body: { total: 0, items: [] } },
      "GET /api/v1/search?jql=type%20%3D%20bug": { status: 200, body: { total: 1, items: [hit("KYB-1", "Login crashes")] } },
      "GET /api/v1/filters": () => ({ status: 200, body: { items: filters } }),
      "POST /api/v1/filters": () => {
        filters = [{ id: "f-1", owner_id: "u", name: "Bugs", jql: "type = bug", shared: true, version: 1, mine: true }];
        return { status: 201, body: filters[0] };
      },
      "DELETE /api/v1/filters/f-1": () => {
        filters = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<SearchPage />);
    expect(await screen.findByText("No saved filters yet.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("JQL"), "type = bug");
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    const results = await screen.findByRole("region", { name: "Results" });
    expect(await within(results).findByRole("link", { name: "KYB-1" })).toHaveAttribute("href", "/projects/KYB?issue=KYB-1");
    expect(within(results).getByText("1 issue")).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText("Save this search as"), "Bugs");
    await userEvent.click(screen.getByLabelText("Share with the workspace"));
    await userEvent.click(screen.getByRole("button", { name: "Save filter" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Filter saved");
    expect(calls.find((c) => c.key === "POST /api/v1/filters")?.body).toEqual({ name: "Bugs", jql: "type = bug", shared: true });
    expect(await screen.findByText("· shared")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Delete filter Bugs" }));
    expect(await screen.findByText("No saved filters yet.")).toBeInTheDocument();
  });

  it("explains invalid JQL and offers examples", async () => {
    mockApi({
      "GET /api/v1/search?jql=": { status: 200, body: { total: 0, items: [] } },
      "GET /api/v1/search?jql=project%20%3D": {
        status: 422, body: { type: "about:blank", title: "t", status: 422, code: "SEARCH_INVALID_JQL", detail: "JQL at 9: expected a value" },
      },
      "GET /api/v1/filters": { status: 200, body: { items: [] } },
      "GET /api/v1/search?jql=sprint%20in%20openSprints()%20AND%20statusCategory%20!%3D%20done": { status: 200, body: { total: 2, items: [hit("KYB-1", "a"), hit("KYB-2", "b")] } },
    });
    renderWithProviders(<SearchPage />);
    await userEvent.type(screen.getByLabelText("JQL"), "project =");
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("JQL at 9: expected a value");
    await userEvent.clear(screen.getByLabelText("JQL"));
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    await userEvent.click(await screen.findByRole("button", { name: "sprint in openSprints() AND statusCategory != done" }));
    await waitFor(() => expect(screen.getByText("2 issues")).toBeInTheDocument());
  });

  it("autocompletes JQL fields and the project's labels", async () => {
    mockApi({
      "GET /api/v1/search?jql=": { status: 200, body: { total: 0, items: [] } },
      "GET /api/v1/filters": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/labels": { status: 200, body: { items: ["backend", "ui"] } },
    });
    renderWithProviders(<SearchPage />);
    const input = screen.getByRole("combobox", { name: "JQL" });
    await userEvent.type(input, "project = KYB AND lab");
    await userEvent.click(await screen.findByRole("option", { name: "labels" }));
    expect(input).toHaveValue("project = KYB AND labels ");
    await userEvent.type(input, "= ");
    expect(await screen.findByRole("option", { name: "backend" })).toBeInTheDocument();
    await userEvent.keyboard("{ArrowDown}{ArrowDown}{Enter}");
    expect(input).toHaveValue("project = KYB AND labels = ui ");
  });
});
