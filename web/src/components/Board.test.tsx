import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Issue } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { Board } from "./Board";

const issue = (n: number, status: Issue["status"], title = `Issue ${n}`): Issue => ({
  id: `i-${n}`, key: `KYB-${n}`, title, type: "task", status,
  description: "", priority: "medium", assignee_id: null, reporter_id: null, estimate: null, sprint_id: null, rank: `a${n}`, version: 1,
});

function drag(card: HTMLElement, column: HTMLElement) {
  const data = new Map<string, string>();
  const dataTransfer = {
    setData: (k: string, v: string) => data.set(k, v),
    getData: (k: string) => data.get(k) ?? "",
    effectAllowed: "move",
    dropEffect: "move",
  };
  fireEvent.dragStart(card, { dataTransfer });
  fireEvent.dragOver(column, { dataTransfer });
  fireEvent.drop(column, { dataTransfer });
}

describe("Board (KYB-S14 AC3)", () => {
  it("renders the three workflow columns with their issues", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "todo"), issue(2, "in_progress"), issue(3, "done")] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const todo = await screen.findByRole("region", { name: "To Do" });
    expect(within(todo).getByText("Issue 1")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "In Progress" })).getByText("Issue 2")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Done" })).getByText("Issue 3")).toBeInTheDocument();
  });

  it("dragging a card to another column transitions the issue", async () => {
    let status: Issue["status"] = "todo";
    const calls = mockApi({
      "GET /api/v1/projects/KYB/issues": () => ({ status: 200, body: { items: [issue(1, status)] } }),
      "POST /api/v1/issues/KYB-1/transitions": () => {
        status = "in_progress";
        return { status: 200, body: issue(1, "in_progress") };
      },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const card = await screen.findByText("Issue 1");
    drag(card.closest("article")!, screen.getByRole("region", { name: "In Progress" }));

    await waitFor(() =>
      expect(within(screen.getByRole("region", { name: "In Progress" })).getByText("Issue 1")).toBeInTheDocument(),
    );
    expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ to: "in_progress" });
  });

  it("a rejected move shows the error and the card returns", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "todo")] } },
      "POST /api/v1/issues/KYB-1/transitions": { status: 409, body: { type: "about:blank", title: "Error", status: 400, code: "TEST", detail: "transition not allowed by workflow" } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const card = await screen.findByText("Issue 1");
    drag(card.closest("article")!, screen.getByRole("region", { name: "Done" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("transition not allowed by workflow");
    await waitFor(() =>
      expect(within(screen.getByRole("region", { name: "To Do" })).getByText("Issue 1")).toBeInTheDocument(),
    );
  });

  it("creates an issue from the quick-add form", async () => {
    const items: Issue[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/issues": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/issues": () => {
        items.push(issue(1, "todo", "Write docs"));
        return { status: 201, body: items[0] };
      },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("New issue title"), "Write docs");
    await userEvent.selectOptions(screen.getByLabelText("Type"), "bug");
    await userEvent.click(screen.getByRole("button", { name: "Add issue" }));

    expect(await within(screen.getByRole("region", { name: "To Do" })).findByText("Write docs")).toBeInTheDocument();
    expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ title: "Write docs", type: "bug" });
  });
});

describe("Board with a project workflow (KYB-S43)", () => {
  it("renders one column per workflow status, in workflow order", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/workflow": {
        status: 200,
        body: {
          initial: "open",
          statuses: [
            { id: "open", name: "Open", category: "todo" },
            { id: "review", name: "Code Review", category: "in_progress" },
            { id: "closed", name: "Closed", category: "done" },
          ],
          transitions: [],
        },
      },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "review"), issue(2, "closed")] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const review = await screen.findByRole("region", { name: "Code Review" });
    expect(within(review).getByText("Issue 1")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "Closed" })).getByText("Issue 2")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "To Do" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"))).toEqual(["Open", "Code Review", "Closed"]);
  });
});

describe("Boards with WIP limits (KYB-S44)", () => {
  const boards = {
    items: [
      { id: "b-1", project_key: "KYB", name: "Delivery", kind: "kanban", version: 1, columns: [
        { name: "Backlog", statuses: ["todo"], min: 0, max: 0 },
        { name: "Doing", statuses: ["in_progress", "review"], min: 0, max: 1 },
        { name: "Shipped", statuses: ["done"], min: 0, max: 0 },
      ] },
      { id: "b-2", project_key: "KYB", name: "Triage", kind: "scrum", version: 1, columns: [{ name: "Open", statuses: ["todo"], min: 0, max: 0 }] },
    ],
  };

  it("shows the board's columns, maps several statuses to one column and flags WIP over the limit", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: boards },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "in_progress"), issue(2, "review"), issue(3, "todo")] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const doing = await screen.findByRole("region", { name: "Doing" });
    expect(within(doing).getByText("Issue 1")).toBeInTheDocument();
    expect(within(doing).getByText("Issue 2")).toBeInTheDocument();
    expect(doing).toHaveClass("wip-over");
    expect(within(doing).getByText("2 / 1")).toBeInTheDocument();
    expect(screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"))).toEqual(["Backlog", "Doing", "Shipped"]);
  });

  it("switches between boards", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: boards },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(3, "todo")] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await screen.findByRole("region", { name: "Doing" });
    await userEvent.selectOptions(screen.getByLabelText("Board"), "b-2");
    expect(await screen.findByRole("region", { name: "Open" })).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Doing" })).not.toBeInTheDocument();
  });

  it("dropping on a column moves the issue to the column's first status", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: boards },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(3, "todo")] } },
      "POST /api/v1/issues/KYB-3/transitions": { status: 200, body: issue(3, "in_progress") },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const card = await screen.findByText("Issue 3");
    drag(card.closest("article")!, screen.getByRole("region", { name: "Doing" }));
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("POST"))?.body).toEqual({ to: "in_progress" }));
  });
});

describe("Board → eye health", () => {
  it("moving an issue into a Done-category status announces the finished task", async () => {
    const seen: string[] = [];
    const listener = (e: Event) => seen.push((e as CustomEvent<string>).detail);
    window.addEventListener("kyber:issue-done", listener);
    mockApi({
      "GET /api/v1/projects/KYB/workflow": { status: 200, body: { initial: "todo", statuses: [
        { id: "todo", name: "To Do", category: "todo" }, { id: "shipped", name: "Shipped", category: "done" }], transitions: [] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "todo")] } },
      "POST /api/v1/issues/KYB-1/transitions": { status: 200, body: issue(1, "shipped") },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const card = await screen.findByText("Issue 1");
    drag(card.closest("article")!, screen.getByRole("region", { name: "Shipped" }));
    await waitFor(() => expect(seen).toEqual(["KYB-1"]));
    window.removeEventListener("kyber:issue-done", listener);
  });
});
