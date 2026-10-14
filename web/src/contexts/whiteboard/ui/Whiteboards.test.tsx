import { screen, waitFor, within } from "@testing-library/react";
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

  it("zooms, fits and pans the infinite canvas (S89)", async () => {
    const far = { ...sticky, id: "e-2", x: 3000, y: 2000, text: "Far away" };
    mockApi({
      "GET /api/v1/conversations/c-1/whiteboards": { status: 200, body: { items: [board] } },
      "GET /api/v1/whiteboards/b-1": { status: 200, body: { ...board, elements: [sticky, far] } },
    });
    const user = userEvent.setup();
    renderWithProviders(<Whiteboards conversationId="c-1" me="u-1" names={names} />);
    await user.click(screen.getByRole("button", { name: "Whiteboards" }));
    await user.click(await screen.findByRole("button", { name: "Architecture" }));
    const canvas = await screen.findByRole("img", { name: "Canvas" });
    await within(canvas).findByText("Far away");
    const zoom = screen.getByRole("group", { name: "Zoom" });
    expect(within(zoom).getByText("100%")).toBeInTheDocument();
    expect(canvas.getAttribute("viewBox")).toBe("0 0 1600 900");
    await user.click(within(zoom).getByRole("button", { name: "Zoom in" }));
    expect(within(zoom).getByText("125%")).toBeInTheDocument();
    await user.click(within(zoom).getByRole("button", { name: "Zoom out" }));
    await user.click(within(zoom).getByRole("button", { name: "Zoom out" }));
    expect(within(zoom).getByText("80%")).toBeInTheDocument();
    await user.click(within(zoom).getByRole("button", { name: "Fit to content" }));
    expect(within(zoom).getByText("41%")).toBeInTheDocument(); // both notes are in view
    await user.click(within(zoom).getByRole("button", { name: "Reset zoom" })); // back to 100 %, same centre
    expect(within(zoom).getByText("100%")).toBeInTheDocument();
    expect(canvas.getAttribute("viewBox")?.endsWith(" 1600 900")).toBe(true);
    expect(screen.getByRole("button", { name: "Hand" })).toBeInTheDocument();
  });

  it("selects all, duplicates and deletes as one step (S90)", async () => {
    const note2 = { ...sticky, id: "e-2", x: 300, text: "Queue?" };
    const calls = mockApi({
      "GET /api/v1/conversations/c-1/whiteboards": { status: 200, body: { items: [board] } },
      "GET /api/v1/whiteboards/b-1": { status: 200, body: { ...board, elements: [sticky, note2] } },
      "POST /api/v1/whiteboards/b-1/ops": (init) => {
        const ops = (JSON.parse(String(init?.body)) as { ops: { op: string; element: { id: string } }[] }).ops;
        return { status: 200, body: { results: ops.map((o) => ({ applied: true, error: "", element: { ...sticky, ...o.element, version: 2, deleted: o.op === "delete" } })) } };
      },
    });
    const user = userEvent.setup();
    renderWithProviders(<Whiteboards conversationId="c-1" me="u-1" names={names} />);
    await user.click(screen.getByRole("button", { name: "Whiteboards" }));
    await user.click(await screen.findByRole("button", { name: "Architecture" }));
    const canvas = await screen.findByRole("img", { name: "Canvas" });
    await within(canvas).findByText("Queue?");
    await user.keyboard("{Control>}a{/Control}");
    expect(screen.getByText("2 selected")).toBeInTheDocument();
    expect(within(canvas).getByLabelText("Selection")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Duplicate" }));
    expect(await within(canvas).findAllByText("Queue?")).toHaveLength(2);
    expect(screen.getByText("2 selected")).toBeInTheDocument(); // the copies are selected
    await user.keyboard("{Delete}");
    await waitFor(() => expect(within(canvas).getAllByText("Queue?")).toHaveLength(1));
    await user.click(screen.getByRole("button", { name: "Undo" })); // one step brings both copies back
    expect(await within(canvas).findAllByText("Queue?")).toHaveLength(2);
    expect(calls.filter((c) => c.key === "POST /api/v1/whiteboards/b-1/ops").length).toBeGreaterThanOrEqual(6);
  });

  it("restyles the selection: weight, fill, dashes and sticky colours (S91)", async () => {
    const box = { ...sticky, id: "e-3", kind: "rect", text: "", color: "#0b6fd8" };
    const calls = mockApi({
      "GET /api/v1/conversations/c-1/whiteboards": { status: 200, body: { items: [board] } },
      "GET /api/v1/whiteboards/b-1": { status: 200, body: { ...board, elements: [sticky, box] } },
      "POST /api/v1/whiteboards/b-1/ops": (init) => {
        const ops = (JSON.parse(String(init?.body)) as { ops: { element: Record<string, unknown> }[] }).ops;
        return { status: 200, body: { results: ops.map((o) => ({ applied: true, error: "", element: { ...o.element, version: 2 } })) } };
      },
    });
    const user = userEvent.setup();
    renderWithProviders(<Whiteboards conversationId="c-1" me="u-1" names={names} />);
    await user.click(screen.getByRole("button", { name: "Whiteboards" }));
    await user.click(await screen.findByRole("button", { name: "Architecture" }));
    const canvas = await screen.findByRole("img", { name: "Canvas" });
    await within(canvas).findByText("Cache?");
    await user.keyboard("{Control>}a{/Control}");
    const style = screen.getByRole("group", { name: "Style" });
    await user.click(within(style).getByRole("button", { name: "Bold" }));
    await user.click(within(style).getByRole("button", { name: "Fill" }));
    await user.click(within(style).getByRole("button", { name: "Dashed" }));
    await user.click(within(screen.getByRole("radiogroup", { name: "Sticky colour" })).getByRole("radio", { name: "Sticky #fbcfe8" }));
    const sent = calls.filter((c) => c.key === "POST /api/v1/whiteboards/b-1/ops").flatMap((c) => (c.body as { ops: { element: Record<string, unknown> }[] }).ops.map((o) => o.element));
    await waitFor(() => expect(sent.filter((e) => e.id === "e-3").at(-1)).toMatchObject({ stroke: 3, fill: true, dashed: true }));
    expect(sent.filter((e) => e.id === "e-1").map((e) => e.color)).toEqual(["#fbcfe8"]); // the note only changed colour
    const drawn = canvas.querySelector('rect[data-id="e-3"]');
    await waitFor(() => expect(drawn?.getAttribute("stroke-dasharray")).toBe("10 6"));
    expect(drawn?.getAttribute("fill")).toBe("#0b6fd833");
  });

  it("starts from a template (S92)", async () => {
    const calls = mockApi({
      "GET /api/v1/conversations/c-1/whiteboards": { status: 200, body: { items: [board] } },
      "GET /api/v1/whiteboards/b-1": { status: 200, body: { ...board, elements: [] } },
      "POST /api/v1/whiteboards/b-1/ops": (init) => {
        const ops = (JSON.parse(String(init?.body)) as { ops: { op: string; element: Record<string, unknown> }[] }).ops;
        return { status: 200, body: { results: ops.map((o) => ({ applied: true, error: "", element: { ...o.element, version: 1, deleted: o.op === "delete" } })) } };
      },
    });
    const user = userEvent.setup();
    renderWithProviders(<Whiteboards conversationId="c-1" me="u-1" names={names} />);
    await user.click(screen.getByRole("button", { name: "Whiteboards" }));
    await user.click(await screen.findByRole("button", { name: "Architecture" }));
    const canvas = await screen.findByRole("img", { name: "Canvas" });
    await user.selectOptions(screen.getByLabelText("Insert template"), "retro");
    expect(await within(canvas).findByText("Went well")).toBeInTheDocument();
    expect(within(canvas).getByText("Actions")).toBeInTheDocument();
    expect(screen.getByText("6 selected")).toBeInTheDocument();
    await waitFor(() => expect(calls.filter((c) => c.key === "POST /api/v1/whiteboards/b-1/ops")).toHaveLength(6));
    await user.click(screen.getByRole("button", { name: "Undo" })); // one step removes the whole template
    await waitFor(() => expect(within(canvas).queryByText("Went well")).not.toBeInTheDocument());
  });
});
