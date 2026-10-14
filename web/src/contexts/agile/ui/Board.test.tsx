import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Issue } from "../../issue/application";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
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

describe("Kanban backlog (S48)", () => {
  const flow = (on: boolean) => ({
    id: "b-1", project_key: "KYB", name: "Flow", kind: "kanban", version: 1, kanban_backlog: on, columns: [
      { name: "Backlog", statuses: ["todo"], min: 0, max: 0 },
      { name: "Doing", statuses: ["in_progress"], min: 0, max: 0 },
      { name: "Done", statuses: ["done"], min: 0, max: 0 },
    ],
  });

  it("lists the first column apart and selects issues for development", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: { items: [flow(true)] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "todo"), issue(2, "in_progress")] } },
      "POST /api/v1/issues/KYB-1/transitions": { status: 200, body: issue(1, "in_progress") },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const backlog = await screen.findByRole("region", { name: "Kanban backlog" });
    expect(within(backlog).getByText("Issue 1")).toBeInTheDocument();
    expect(screen.getAllByRole("region").map((r) => r.getAttribute("aria-label"))).toEqual(["Doing", "Done", "Kanban backlog"]);
    await userEvent.click(within(backlog).getByRole("button", { name: "Select KYB-1 for development" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/issues/KYB-1/transitions")?.body).toEqual({ to: "in_progress" }));
  });

  it("is switched on from the board and shows a refusal", async () => {
    let on = false;
    const calls = mockApi({
      "GET /api/v1/projects/KYB/boards": () => ({ status: 200, body: { items: [flow(on)] } }),
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "todo")] } },
      "PUT /api/v1/boards/b-1": () => {
        on = true;
        return { status: 200, body: flow(true) };
      },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await userEvent.click(await screen.findByLabelText("Kanban backlog"));
    expect(await screen.findByRole("region", { name: "Kanban backlog" })).toBeInTheDocument();
    expect(calls.find((c) => c.key === "PUT /api/v1/boards/b-1")?.body).toMatchObject({ kanban_backlog: true, version: 1, name: "Flow" });
  });

  it("explains when only admins may change the board", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/boards": { status: 200, body: { items: [flow(false)] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
      "PUT /api/v1/boards/b-1": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "admins only" } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await userEvent.click(await screen.findByLabelText("Kanban backlog"));
    expect(await screen.findByRole("alert")).toHaveTextContent("admins only");
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

  it("splits the board into swimlanes by assignee or epic and remembers the choice", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/issues": {
        status: 200,
        body: {
          items: [
            { ...issue(1, "todo", "Login form"), assignee_id: "u-1", parent_key: "KYB-9" },
            { ...issue(2, "in_progress", "Logout"), assignee_id: null, parent_key: null },
            { ...issue(9, "todo", "Accounts"), type: "epic" },
          ],
        },
      },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-1", name: "Ali Valiyev", email: "a@x.uz", role: "member" }] } },
    });
    const { unmount } = renderWithProviders(<Board projectKey="KYB" />);
    await screen.findByRole("region", { name: "To Do" });
    await userEvent.selectOptions(screen.getByLabelText("Group"), "assignee");
    const ali = await screen.findByRole("region", { name: "Ali Valiyev" });
    expect(within(within(ali).getByRole("region", { name: "To Do · Ali Valiyev" })).getByText("Login form")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "In Progress · Unassigned" })).getByText("Logout")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("Group"), "epic");
    expect(within(await screen.findByRole("region", { name: "To Do · KYB-9 Accounts" })).getByText("Login form")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "No epic" })).toBeInTheDocument();
    unmount();

    renderWithProviders(<Board projectKey="KYB" />);
    expect(await screen.findByRole("region", { name: "No epic" })).toBeInTheDocument();
    localStorage.removeItem("kyber.swimlanes.KYB");
  });

  it("filters the board from the toolbar: text, people, epic, type, and clears", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/issues": {
        status: 200,
        body: {
          items: [
            { ...issue(1, "todo", "Login form"), type: "story", assignee_id: "u-1", parent_key: "KYB-9" },
            { ...issue(2, "todo", "Crash on save"), type: "bug" },
            { ...issue(9, "todo", "Accounts"), type: "epic" },
          ],
        },
      },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [{ user_id: "u-1", name: "Ali Valiyev", email: "a@x.uz", role: "member" }] } },
      "GET /api/v1/projects/KYB/versions": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    const todo = await screen.findByRole("region", { name: "To Do" });
    await userEvent.type(screen.getByLabelText("Search board"), "crash");
    expect(within(todo).queryByText("Login form")).not.toBeInTheDocument();
    expect(within(todo).getByText("Crash on save")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
    expect(within(todo).getByText("Login form")).toBeInTheDocument();

    await userEvent.click(await screen.findByRole("button", { name: "Ali Valiyev" }));
    expect(screen.getByRole("button", { name: "Ali Valiyev" })).toHaveAttribute("aria-pressed", "true");
    expect(within(todo).queryByText("Crash on save")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));

    await userEvent.click(within(screen.getByRole("group", { name: "Filter by kind" })).getByLabelText("bug"));
    expect(within(todo).getByText("Crash on save")).toBeInTheDocument();
    expect(within(todo).queryByText("Login form")).not.toBeInTheDocument();
    await userEvent.click(within(screen.getByRole("group", { name: "Filter by kind" })).getByLabelText("bug"));
    await userEvent.click(within(screen.getByRole("group", { name: "Filter by Epic" })).getByLabelText("KYB-9 Accounts"));
    expect(within(todo).getByText("Login form")).toBeInTheDocument();
    expect(within(todo).queryByText("Crash on save")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Board insights" })).toHaveAttribute("href", "/projects/KYB/reports");
  });

  it("completes the active sprint from the toolbar", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [{ id: "s-1", project_key: "KYB", name: "Sprint 1", goal: "", state: "active", started_at: null, completed_at: null }] } },
      "GET /api/v1/projects/KYB/issues?sprint=s-1": { status: 200, body: { items: [issue(1, "todo")] } },
      "POST /api/v1/sprints/s-1/complete": { status: 200, body: { sprint: {}, completed: 0, returned: 1 } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await userEvent.click(await screen.findByRole("button", { name: "Complete sprint" }));
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/v1/sprints/s-1/complete")).toBe(true));
  });
});
