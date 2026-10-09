import { describe, expect, it } from "vitest";
import { LIBRARY, positionAt, stepAt, timeline } from "./exercises";

describe("eye exercises (ported from Sharingan)", () => {
  it("builds a timeline with cumulative start times", () => {
    const tl = timeline(LIBRARY);
    expect(tl.steps[0]).toMatchObject({ exercise: "20-20-20", start: 0, hold: 20, direction: "far" });
    expect(tl.steps[1]?.start).toBe(20);
    expect(tl.total).toBe(25 + 4 * 4 + 4 * 2 + 4 * 4 + 2 + 6 * 3 + 8 + 4);
    expect(tl.steps.every((s) => s.instruction.length > 0)).toBe(true);
  });

  it("finds the step at a time and reports progress within it", () => {
    const tl = timeline(LIBRARY);
    expect(stepAt(tl, 0)?.step.direction).toBe("far");
    expect(stepAt(tl, 21)?.step.direction).toBe("closed");
    const s = stepAt(tl, 26);
    expect(s?.step.direction).toBe("right");
    expect(s?.progress).toBeCloseTo(1 / 4);
    expect(stepAt(tl, tl.total)).toBeNull();
    expect(stepAt(tl, -1)?.step.direction).toBe("far");
  });

  it("places the guide for each direction and traces circles and a figure 8", () => {
    expect(positionAt("center", 0)).toEqual({ x: 0, y: 0 });
    expect(positionAt("right", 0.5)).toEqual({ x: 1, y: 0 });
    expect(positionAt("up_left", 0.5)).toEqual({ x: -1, y: -1 });
    expect(positionAt("down", 0.9)).toEqual({ x: 0, y: 1 });
    const c0 = positionAt("circle_cw", 0);
    const cq = positionAt("circle_cw", 0.25);
    expect(c0.x).toBeCloseTo(0);
    expect(c0.y).toBeCloseTo(-1);
    expect(cq.x).toBeCloseTo(1); // clockwise: top → right
    expect(positionAt("circle_ccw", 0.25).x).toBeCloseTo(-1);
    const f = positionAt("figure8", 0.125);
    expect(Math.abs(f.x)).toBeLessThanOrEqual(1);
    expect(Math.abs(f.y)).toBeLessThanOrEqual(1);
    expect(positionAt("far", 0.3)).toEqual({ x: 0, y: 0 });
    expect(positionAt("unknown", 0.3)).toEqual({ x: 0, y: 0 });
  });
});
