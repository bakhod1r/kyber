import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Issue } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { Board } from "./Board";

const issue = (n: number, status: Issue["status"], title = `Issue ${n}`): Issue => ({
  id: `i-${n}`, key: `KYB-${n}`, title, type: "task", status,
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
      "POST /api/v1/issues/KYB-1/transitions": { status: 409, body: { error: "transition not allowed by workflow" } },
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
