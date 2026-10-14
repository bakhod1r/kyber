import { describe, expect, it } from "vitest";
import { HANDLES } from "./model";
import { COLORS, erased, fitImage, imageURL, look, ordered, restyle, STICKY, STICKY_COLORS, template, toBack, toFront } from "./model";
import { applyLocal, bounds, box, duplicate, fit, fromClipboard, HOME, inverseAll, MIN_SIZE, moveBy, moveEnd, newer, type Element, hit, inverse, linkedIssue, onScreen, panBy, reconcile, resize, stickyToIssue, toClipboard, viewBox, within, zoomAt } from "./model";

const rect: Element = { id: "r", kind: "rect", x: 10, y: 10, w: 100, h: 50, points: [], text: "", color: "#000000", version: 1 };
const line: Element = { ...rect, id: "l", kind: "line", x: 0, y: 0, w: 0, h: 0, points: [{ x: 0, y: 0 }, { x: 100, y: 100 }] };

describe("whiteboard domain", () => {
  it("applies locally and takes the server's word", () => {
    let els = applyLocal({}, { op: "add", element: rect });
    els = applyLocal(els, { op: "update", element: { ...rect, x: 50 } });
    expect(els.r?.x).toBe(50);
    els = reconcile(els, [{ element: { ...rect, x: 20, version: 2 } }]); // our move lost: the server says 20
    expect(els.r).toMatchObject({ x: 20, version: 2 });
    els = reconcile(els, [{ element: { ...rect, deleted: true } }]);
    expect(els.r).toBeUndefined();
  });

  it("keeps only changes newer than what we know", () => {
    const known = { r: rect };
    expect(newer(known, [rect, { ...rect, version: 2 }, { ...rect, id: "n" }]).map((e) => `${e.id}${e.version}`)).toEqual(["r2", "n1"]);
  });

  it("inverts changes for undo", () => {
    expect(inverse({ op: "add", element: rect }, undefined)).toEqual({ op: "delete", element: rect });
    expect(inverse({ op: "delete", element: rect }, rect)).toEqual({ op: "add", element: { ...rect, version: 0 } });
    expect(inverse({ op: "update", element: { ...rect, x: 99 } }, rect)).toEqual({ op: "update", element: rect });
  });

  it("normalises boxes and hit-tests shapes and strokes", () => {
    expect(box(50, 40, 10, 0)).toEqual({ x: 10, y: 0, w: 40, h: 40 });
    expect(hit(rect, { x: 60, y: 30 })).toBe(true);
    expect(hit(rect, { x: 200, y: 30 })).toBe(false);
    expect(hit(line, { x: 51, y: 49 })).toBe(true);
    expect(hit(line, { x: 80, y: 10 })).toBe(false);
  });
});

describe("sticky notes become issues", () => {
  it("splits title and description and remembers the issue", () => {
    expect(stickyToIssue("  Cache the board\nUse Redis\nor memory ")).toEqual({ title: "Cache the board", description: "Use Redis\nor memory" });
    expect(linkedIssue("Cache the board\n→ KYB-12")).toBe("KYB-12");
    expect(linkedIssue("Cache the board")).toBeUndefined();
  });
});

describe("viewport (S89)", () => {
  const page = { w: 1600, h: 900 };
  it("zooms around a point, within 10 %–400 %", () => {
    const v = zoomAt(HOME, 2, { x: 800, y: 450 });
    expect(v).toEqual({ x: 400, y: 225, zoom: 2 });
    expect(zoomAt(v, 0.5, { x: 800, y: 450 })).toEqual(HOME); // the point under the pointer stays put
    expect(zoomAt(HOME, 100, { x: 0, y: 0 }).zoom).toBe(4);
    expect(zoomAt(HOME, 0.001, { x: 0, y: 0 }).zoom).toBe(0.1);
  });

  it("pans by a distance on screen (in page units)", () => {
    expect(panBy({ x: 0, y: 0, zoom: 2 }, 100, -50)).toEqual({ x: -50, y: 25, zoom: 2 });
  });

  it("maps the view to an SVG viewBox and back to page percentages", () => {
    expect(viewBox({ x: 10, y: 20, zoom: 2 }, page)).toBe("10 20 800 450");
    expect(onScreen({ x: 10, y: 20, zoom: 2 }, page, { x: 410, y: 245 })).toEqual({ left: 50, top: 50 });
  });

  it("fits every element (with a margin) and knows their bounds", () => {
    const far: Element = { ...rect, id: "far", x: 3000, y: 2000, w: 200, h: 100 };
    expect(bounds([rect, line, far])).toEqual({ x: 0, y: 0, w: 3200, h: 2100 });
    expect(bounds([])).toBeUndefined();
    const v = fit([rect, far], page);
    expect(v.zoom).toBeCloseTo(900 / (2090 + 2 * 40), 5); // height decides here
    expect(fit([], page)).toEqual(HOME);
    expect(fit([{ ...rect, w: 1, h: 1 }], page).zoom).toBe(4); // a tiny drawing is not blown up past 400 %
  });
});

describe("editing several elements (S90)", () => {
  it("resizes a box by any handle, never below the minimum or inside out", () => {
    expect(resize(rect, "se", 20, 10)).toMatchObject({ x: 10, y: 10, w: 120, h: 60 });
    expect(resize(rect, "nw", 20, 10)).toMatchObject({ x: 30, y: 20, w: 80, h: 40 });
    expect(resize(rect, "n", 0, -5)).toMatchObject({ y: 5, h: 55, x: 10, w: 100 });
    expect(resize(rect, "e", -500, 0)).toMatchObject({ x: 10, w: MIN_SIZE });
    expect(resize(rect, "w", 500, 0)).toMatchObject({ x: 110 - MIN_SIZE, w: MIN_SIZE });
  });

  it("drags a line's end", () => {
    expect(moveEnd(line, 1, { x: 150, y: 50 }).points).toEqual([{ x: 0, y: 0 }, { x: 150, y: 50 }]);
    expect(moveEnd({ ...line, x: 10, y: 10 }, 0, { x: 0, y: 0 }).points[0]).toEqual({ x: -10, y: -10 });
  });

  it("moves and nudges a selection", () => {
    expect(moveBy([rect, line], 5, -5).map((e) => [e.x, e.y])).toEqual([[15, 5], [5, -5]]);
  });

  it("selects what a marquee fully covers", () => {
    const els = [rect, line, { ...rect, id: "far", x: 500 }];
    expect(within(els, { x: 0, y: 0, w: 200, h: 200 }).map((e) => e.id)).toEqual(["r", "l"]);
    expect(within(els, { x: 0, y: 0, w: 50, h: 50 })).toEqual([]);
  });

  it("copies elements with fresh ids, offset, as new", () => {
    let n = 0;
    const copies = duplicate([{ ...rect, version: 7, updated_at: "2026-10-13T09:00:00Z", updated_by: "u-1" }], 20, () => `c${++n}`);
    expect(copies).toEqual([{ ...rect, id: "c1", x: 30, y: 30, version: 0 }]); // new: drawn on top until the server stamps it
  });

  it("round-trips the clipboard and ignores anything else", () => {
    const text = toClipboard([rect]);
    expect(fromClipboard(text)).toEqual([rect]);
    expect(fromClipboard("hello")).toEqual([]);
    expect(fromClipboard('{"kyber":"other"}')).toEqual([]);
    expect(fromClipboard('{"kyber":"whiteboard","elements":[{"id":1}]}')).toEqual([]);
  });

  it("undoes a batch in reverse order", () => {
    const before = { r: rect };
    expect(inverseAll([{ op: "update", element: { ...rect, x: 1 } }, { op: "add", element: line }], before)).toEqual([
      { op: "delete", element: line },
      { op: "update", element: rect },
    ]);
  });
});

describe("styles (S91)", () => {
  it("maps a style to how a shape is drawn", () => {
    expect(look(rect)).toEqual({ strokeWidth: 2, fill: "none", dash: undefined });
    expect(look({ ...rect, stroke: 3, fill: true, dashed: true, color: "#dc2626" })).toEqual({ strokeWidth: 6, fill: "#dc262633", dash: "10 6" });
    expect(look({ ...rect, stroke: 1 }).strokeWidth).toBe(1);
    expect(look({ ...line, fill: true }).fill).toBe("none"); // lines are never filled
    expect(COLORS).toHaveLength(12);
    expect(STICKY_COLORS[0]).toBe(STICKY);
  });

  it("restyles a selection, leaving what a style does not apply to", () => {
    const note: Element = { ...rect, id: "n", kind: "sticky", color: STICKY };
    const out = restyle([rect, line, note], { stroke: 3, fill: true });
    expect(out.map((e) => [e.id, e.stroke, e.fill])).toEqual([["r", 3, true], ["l", 3, undefined]]); // a sticky has no stroke style
  });
});

describe("templates (S92)", () => {
  it("builds each template as ordinary new elements around a point", () => {
    let n = 0;
    const id = () => `t${++n}`;
    const retro = template("retro", { x: 1000, y: 500 }, id);
    expect(retro.filter((e) => e.kind === "rect")).toHaveLength(3);
    expect(retro.filter((e) => e.kind === "text").map((e) => e.text)).toEqual(["Went well", "To improve", "Actions"]);
    expect(retro.every((e) => e.version === 0 && e.id.startsWith("t"))).toBe(true);
    const b = bounds(retro)!;
    expect(Math.abs(b.x + b.w / 2 - 1000)).toBeLessThan(1); // centred on the point
    expect(Math.abs(b.y + b.h / 2 - 500)).toBeLessThan(1);
    expect(template("kanban", { x: 0, y: 0 }).filter((e) => e.kind === "text").map((e) => e.text)).toEqual(["To do", "Doing", "Done"]);
    expect(template("swot", { x: 0, y: 0 }).filter((e) => e.kind === "text").map((e) => e.text)).toEqual(["Strengths", "Weaknesses", "Opportunities", "Threats"]);
    expect(new Set(template("swot", { x: 0, y: 0 }).map((e) => e.id)).size).toBe(8);
  });
});

describe("eraser and order (S93)", () => {
  it("draws by stored order first, then by last change", () => {
    const a: Element = { ...rect, id: "a", updated_at: "2026-10-13T09:00:00Z" };
    const b: Element = { ...rect, id: "b", updated_at: "2026-10-13T10:00:00Z" };
    expect(ordered({ a, b }).map((e) => e.id)).toEqual(["a", "b"]);
    expect(ordered({ a: { ...a, z: 1 }, b }).map((e) => e.id)).toEqual(["b", "a"]);
  });

  it("brings a selection to the front or sends it to the back, keeping its own order", () => {
    const els = { a: { ...rect, id: "a", z: 0 }, b: { ...rect, id: "b", z: 5 }, c: { ...rect, id: "c", z: 2 } };
    expect(toFront([els.a, els.c], els).map((e) => [e.id, e.z])).toEqual([["a", 6], ["c", 7]]);
    expect(toBack([els.b, els.c], els).map((e) => [e.id, e.z])).toEqual([["c", -2], ["b", -1]]);
    expect(toFront([], els)).toEqual([]);
  });

  it("erases what a stroke of the eraser passes over", () => {
    const els = [rect, line, { ...rect, id: "far", x: 500 }];
    expect(erased(els, [{ x: 0, y: 0 }, { x: 20, y: 20 }]).map((e) => e.id).sort()).toEqual(["l", "r"]);
    expect(erased(els, [])).toEqual([]);
  });
});

describe("images (S96)", () => {
  it("sizes a picture to fit, keeping its proportions", () => {
    expect(fitImage(1920, 1080)).toEqual({ w: 480, h: 270 });
    expect(fitImage(300, 600)).toEqual({ w: 240, h: 480 });
    expect(fitImage(100, 50)).toEqual({ w: 100, h: 50 }); // small pictures keep their size
    expect(fitImage(0, 0)).toEqual({ w: 320, h: 240 }); // unknown size
  });

  it("points an image element at the board's picture", () => {
    expect(imageURL("b 1", { text: "i-1" })).toBe("/api/v1/whiteboards/b%201/images/i-1");
  });

  it("styles never apply to pictures", () => {
    const pic: Element = { ...rect, id: "p", kind: "image", text: "i-1" };
    expect(restyle([pic, rect], { stroke: 3 }).map((e) => e.id)).toEqual(["r"]);
  });
});

describe("handles (S96)", () => {
  it("draws the south-east handle last, on top when a tiny element's handles overlap", () => {
    expect(HANDLES.at(-1)).toBe("se");
    expect(new Set(HANDLES).size).toBe(8);
  });
});
