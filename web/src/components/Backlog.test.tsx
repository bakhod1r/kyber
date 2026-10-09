import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Issue, Sprint } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { Backlog } from "./Backlog";
import { Board } from "./Board";

const issue = (n: number, sprint: string | null, status: Issue["status"] = "todo"): Issue => ({
  id: `i-${n}`, key: `KYB-${n}`, title: `Issue ${n}`, type: "task", status,
  description: "", priority: "medium", assignee_id: null, reporter_id: null, estimate: null, sprint_id: sprint, rank: `a${n}`, version: 1,
});
const sprint = (id: string, name: string, state: Sprint["state"], goal = ""): Sprint => ({
  id, project_key: "KYB", name, goal, state, started_at: state === "planned" ? null : "2026-03-01T09:00:00Z", completed_at: null,
});

function drag(from: HTMLElement, to: HTMLElement) {
  const data = new Map<string, string>();
  const dataTransfer = { setData: (k: string, v: string) => data.set(k, v), getData: (k: string) => data.get(k) ?? "", effectAllowed: "move", dropEffect: "move" };
  fireEvent.dragStart(from, { dataTransfer });
  fireEvent.dragOver(to, { dataTransfer });
  fireEvent.drop(to, { dataTransfer });
}

describe("Backlog (KYB-S20)", () => {
  it("AC1: shows sprint sections above the backlog, in rank order", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "planned", "Ship it")] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(2, "s-1"), issue(1, null), issue(3, null)] } },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const s1 = await screen.findByRole("region", { name: "Sprint 1" });
    expect(within(s1).getByText("Ship it")).toBeInTheDocument();
    expect(within(s1).getByText("Issue 2")).toBeInTheDocument();
    const backlog = screen.getByRole("region", { name: "Backlog" });
    const rows = within(backlog).getAllByRole("listitem").map((li) => li.textContent);
    expect(rows[0]).toContain("Issue 1");
    expect(rows[1]).toContain("Issue 3");
  });

  it("AC1: dropping a backlog issue on a sprint row moves it into the sprint before that row", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "planned")] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(2, "s-1"), issue(1, null)] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: issue(1, "s-1") },
      "POST /api/v1/issues/KYB-1/rank": { status: 200, body: issue(1, "s-1") },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const source = (await screen.findByText("Issue 1")).closest("li")!;
    const target = screen.getByText("Issue 2").closest("li")!;
    drag(source, target);
    await waitFor(() => expect(calls.some((c) => c.key === "POST /api/v1/issues/KYB-1/rank")).toBe(true));
    expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({ version: 1, sprint_id: "s-1" });
    expect(calls.find((c) => c.key === "POST /api/v1/issues/KYB-1/rank")?.body).toEqual({ before: "KYB-2" });
  });

  it("AC1: reordering inside the same section only re-ranks", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, null), issue(2, null)] } },
      "POST /api/v1/issues/KYB-2/rank": { status: 200, body: issue(2, null) },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    drag((await screen.findByText("Issue 2")).closest("li")!, screen.getByText("Issue 1").closest("li")!);
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/issues/KYB-2/rank")?.body).toEqual({ before: "KYB-1" }));
    expect(calls.some((c) => c.key.startsWith("PATCH"))).toBe(false);
  });

  it("AC1: dropping on a section's empty area moves the issue there", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "planned")] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, null)] } },
      "PATCH /api/v1/issues/KYB-1": { status: 200, body: issue(1, "s-1") },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    drag((await screen.findByText("Issue 1")).closest("li")!, screen.getByRole("region", { name: "Sprint 1" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-1")?.body).toEqual({ version: 1, sprint_id: "s-1" }));
  });

  it("AC2: creates and starts a sprint; start is disabled while another is active", async () => {
    const sprints = [sprint("s-1", "Sprint 1", "active"), sprint("s-2", "Sprint 2", "planned")];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": () => ({ status: 200, body: { items: sprints } }),
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/sprints": () => {
        sprints.push(sprint("s-3", "Sprint 3", "planned"));
        return { status: 201, body: sprints[2] };
      },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const s2 = await screen.findByRole("region", { name: "Sprint 2" });
    expect(within(s2).getByRole("button", { name: "Start sprint" })).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Sprint name"), "Sprint 3");
    await userEvent.click(screen.getByRole("button", { name: "Create sprint" }));
    expect(await screen.findByRole("region", { name: "Sprint 3" })).toBeInTheDocument();
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/sprints")?.body).toEqual({ name: "Sprint 3", goal: "" });
  });

  it("AC2: completing a sprint reports what returned to the backlog", async () => {
    let state: Sprint["state"] = "active";
    mockApi({
      "GET /api/v1/projects/KYB/sprints": () => ({ status: 200, body: { items: state === "active" ? [sprint("s-1", "Sprint 1", "active")] : [] } }),
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [issue(1, "s-1", "done"), issue(2, "s-1")] } },
      "POST /api/v1/sprints/s-1/complete": () => {
        state = "closed";
        return { status: 200, body: { sprint: sprint("s-1", "Sprint 1", "closed"), completed: 1, returned: 1 } };
      },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const s1 = await screen.findByRole("region", { name: "Sprint 1" });
    await userEvent.click(within(s1).getByRole("button", { name: "Complete sprint" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Sprint 1 completed: 1 done, 1 returned to the backlog.");
  });
});

describe("Board with an active sprint (KYB-S20 AC3)", () => {
  it("shows only the active sprint's issues and its goal", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "active", "Ship it")] } },
      "GET /api/v1/projects/KYB/issues?sprint=s-1": { status: 200, body: { items: [issue(2, "s-1")] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    expect(await screen.findByText("Issue 2")).toBeInTheDocument();
    expect(screen.getByText(/Sprint 1/)).toBeInTheDocument();
    expect(screen.getByText(/Ship it/)).toBeInTheDocument();
    expect(calls.some((c) => c.key === "GET /api/v1/projects/KYB/issues")).toBe(false);
  });
});

describe("Quick-add during an active sprint (Jira parity)", () => {
  it("adds the new issue to the active sprint so it stays on the board", async () => {
    const items: Issue[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "active")] } },
      "GET /api/v1/projects/KYB/issues?sprint=s-1": () => ({ status: 200, body: { items } }),
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/issues": { status: 201, body: issue(7, null) },
      "PATCH /api/v1/issues/KYB-7": () => {
        items.push(issue(7, "s-1"));
        return { status: 200, body: items[0] };
      },
    });
    renderWithProviders(<Board projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("New issue title"), "Hotfix");
    await userEvent.click(screen.getByRole("button", { name: "Add issue" }));
    expect(await within(screen.getByRole("region", { name: "To Do" })).findByText("Issue 7")).toBeInTheDocument();
    expect(calls.find((c) => c.key === "PATCH /api/v1/issues/KYB-7")?.body).toEqual({ version: 1, sprint_id: "s-1" });
  });
});

describe("Estimates and sprint dates (KYB-S28 AC3)", () => {
  it("backlog rows show story points and sections total them", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "planned")] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [{ ...issue(1, "s-1"), estimate: 3 }, { ...issue(2, "s-1"), estimate: 2.5 }, issue(3, null)] } },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const s1 = await screen.findByRole("region", { name: "Sprint 1" });
    expect(within(s1).getByLabelText("Story points: 3")).toBeInTheDocument();
    expect(within(s1).getByText("5.5 pts")).toBeInTheDocument();
  });

  it("starting a sprint sends the chosen end date", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/sprints": { status: 200, body: { items: [sprint("s-1", "Sprint 1", "planned")] } },
      "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } },
      "POST /api/v1/sprints/s-1/start": { status: 200, body: sprint("s-1", "Sprint 1", "active") },
    });
    renderWithProviders(<Backlog projectKey="KYB" />);
    const s1 = await screen.findByRole("region", { name: "Sprint 1" });
    const end = within(s1).getByLabelText("End date");
    await userEvent.clear(end);
    await userEvent.type(end, "2026-12-24");
    await userEvent.click(within(s1).getByRole("button", { name: "Start sprint" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/sprints/s-1/start")?.body).toEqual({ ends_at: "2026-12-24T23:59:59.000Z" }));
  });
});
