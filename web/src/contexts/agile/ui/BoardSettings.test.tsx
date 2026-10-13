import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { BoardSettings } from "./BoardSettings";

const workflow = {
  initial: "todo",
  statuses: [
    { id: "todo", name: "To Do", category: "todo" },
    { id: "review", name: "Review", category: "in_progress" },
    { id: "done", name: "Done", category: "done" },
  ],
  transitions: [],
};
const board = {
  id: "b-1", project_key: "KYB", name: "Flow", kind: "kanban", version: 1, kanban_backlog: false,
  columns: [
    { name: "To Do", statuses: ["todo"], min: 0, max: 0 },
    { name: "Done", statuses: ["done"], min: 0, max: 0 },
  ],
};

describe("BoardSettings (S44)", () => {
  it("adds a column, maps a status into it, sets WIP limits and saves", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: { items: [board] } },
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "PUT /api/v1/boards/b-1": { status: 200, body: { ...board, version: 2 } },
    });
    renderWithProviders(<BoardSettings projectKey="KYB" />);
    expect(await screen.findByLabelText("Column for Review")).toHaveValue("-1");
    await userEvent.click(screen.getByRole("button", { name: "Add column" }));
    await userEvent.clear(screen.getByLabelText("Column 3 name"));
    await userEvent.type(screen.getByLabelText("Column 3 name"), "Review");
    await userEvent.click(screen.getByRole("button", { name: "Move Review left" }));
    await userEvent.selectOptions(screen.getByLabelText("Column for Review"), "1");
    await userEvent.type(screen.getByLabelText("Review max"), "2");
    await userEvent.click(screen.getByRole("button", { name: "Save board" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PUT /api/v1/boards/b-1")?.body).toEqual({
      version: 1, name: "Flow", kind: "kanban", kanban_backlog: false,
      columns: [
        { name: "To Do", statuses: ["todo"], min: 0, max: 0 },
        { name: "Review", statuses: ["review"], min: 0, max: 2 },
        { name: "Done", statuses: ["done"], min: 0, max: 0 },
      ],
    }));
    expect(await screen.findByRole("status")).toHaveTextContent("Board saved");
  });

  it("blocks invalid boards, discards changes and shows refusals", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: { items: [board, { ...board, id: "b-2", name: "Other" }] } },
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: workflow },
      "POST /api/v1/projects/KYB/boards": { status: 201, body: { ...board, id: "b-3", name: "Ops" } },
      "DELETE /api/v1/boards/b-1": { status: 409, body: { type: "about:blank", title: "t", status: 409, code: "BOARD_LAST", detail: "keep one board" } },
      "PUT /api/v1/boards/b-2": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "admins only" } },
    });
    renderWithProviders(<BoardSettings projectKey="KYB" />);
    await userEvent.clear(await screen.findByLabelText("Column 1 name"));
    expect(within(screen.getByRole("list", { name: "Problems" })).getByText("Every column needs a name.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save board" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "Discard changes" }));
    expect(screen.queryByRole("list", { name: "Problems" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remove column Done" }));
    await userEvent.selectOptions(screen.getByLabelText("Kind"), "scrum");
    expect(screen.queryByLabelText("Kanban backlog (first column)")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Delete Flow" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("keep one board");

    await userEvent.type(screen.getByLabelText("New board name"), "Ops");
    await userEvent.click(screen.getByRole("button", { name: "Create board" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Board created");

    await userEvent.selectOptions(screen.getByLabelText("Board"), "b-2");
    await userEvent.click(screen.getByRole("button", { name: "Save board" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("admins only");
  });
});
