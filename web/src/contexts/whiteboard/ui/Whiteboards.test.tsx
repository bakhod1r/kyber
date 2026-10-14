import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { Whiteboards } from "./Whiteboards";

const names = new Map([["u-1", "Ali"], ["u-2", "Bobur"]]);
const board = { id: "b-1", conversation_id: "c-1", title: "Architecture", created_by: "u-2", created_at: "2026-10-13T09:00:00Z" };
const sticky = { id: "e-1", kind: "sticky", x: 10, y: 10, w: 180, h: 120, points: [], text: "Cache?", color: "#fde68a", version: 1, updated_at: "2026-10-13T09:00:00Z" };

describe("Whiteboards (S59)", () => {
  it("lists, creates and opens a whiteboard with its elements", async () => {
    const calls = mockApi({
      "GET /api/v1/conversations/c-1/whiteboards": { status: 200, body: { items: [board] } },
      "POST /api/v1/conversations/c-1/whiteboards": { status: 201, body: { ...board, id: "b-2", title: "Retro", elements: [] } },
      "GET /api/v1/whiteboards/b-1": { status: 200, body: { ...board, elements: [sticky] } },
      "GET /api/v1/whiteboards/b-2": { status: 200, body: { ...board, id: "b-2", title: "Retro", elements: [] } },
    });
    const user = userEvent.setup();
    renderWithProviders(<Whiteboards conversationId="c-1" me="u-1" names={names} />);
    await user.click(screen.getByRole("button", { name: "Whiteboards" }));
    const list = await screen.findByRole("region", { name: "Whiteboards" });
    expect(await within(list).findByText(/by Bobur/)).toBeInTheDocument();

    await user.click(within(list).getByRole("button", { name: "Architecture" }));
    const canvas = await screen.findByRole("img", { name: "Canvas" });
    expect(await within(canvas).findByText("Cache?")).toBeInTheDocument();
    expect(screen.getByRole("toolbar", { name: "Drawing tools" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Undo" })).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Close whiteboard" }));
    await user.type(await screen.findByLabelText("Whiteboard title"), "Retro");
    await user.click(screen.getByRole("button", { name: "New whiteboard" }));
    expect(calls.find((c) => c.key === "POST /api/v1/conversations/c-1/whiteboards")?.body).toEqual({ title: "Retro" });
    expect(await screen.findByRole("region", { name: "Whiteboard Retro" })).toBeInTheDocument();
  });
});
