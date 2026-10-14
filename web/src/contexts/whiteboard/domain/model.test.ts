import { describe, expect, it } from "vitest";
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
