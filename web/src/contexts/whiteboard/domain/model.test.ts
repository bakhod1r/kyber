import { describe, expect, it } from "vitest";
import { applyLocal, box, type Element, hit, inverse, reconcile } from "./model";

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
