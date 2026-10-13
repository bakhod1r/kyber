// Eye-break exercises, ported from Sharingan (github.com/bakhod1r/sharingan, BreakExercise.swift).
// Pure data and geometry so the break screen stays a thin view.

export type Step = { direction: string; hold: number; instruction: string };
export type Exercise = { name: string; steps: Step[] };

const LABEL: Record<string, string> = {
  up: "up", down: "down", left: "left", right: "right", center: "at the center",
  up_left: "up and to the left", up_right: "up and to the right", down_left: "down and to the left", down_right: "down and to the right",
};
const step = (direction: string, hold: number, instruction = ""): Step => ({
  direction, hold: Math.max(0.5, hold), instruction: instruction || `Look ${LABEL[direction] ?? direction}`,
});

export const LIBRARY: Exercise[] = [
  { name: "20-20-20", steps: [step("far", 20, "Look at something 20 feet (6 m) away for 20 seconds"), step("closed", 5, "Close your eyes and breathe")] },
  {
    name: "Gaze exercise",
    steps: [
      step("right", 4), step("center", 2), step("left", 4), step("center", 2),
      step("up", 4), step("center", 2), step("down", 4), step("center", 2),
      step("up_right", 4), step("down_left", 4), step("up_left", 4), step("down_right", 4), step("center", 2),
      step("circle_cw", 6, "Roll your eyes slowly clockwise"),
      step("circle_ccw", 6, "Now roll them counter-clockwise"),
      step("figure8", 6, "Trace a figure 8 with your eyes"),
    ],
  },
  { name: "Blink exercise", steps: [step("blink", 8, "Blink quickly 8 times"), step("closed", 4, "Now keep your eyes softly closed for 4 seconds")] },
];

export type TimedStep = Step & { exercise: string; start: number };
export type Timeline = { steps: TimedStep[]; total: number };

export function timeline(exercises: Exercise[]): Timeline {
  const steps: TimedStep[] = [];
  let t = 0;
  for (const e of exercises) {
    for (const s of e.steps) {
      steps.push({ ...s, exercise: e.name, start: t });
      t += s.hold;
    }
  }
  return { steps, total: t };
}

/** The step running at second t (clamped at 0), with progress 0..1 inside it; null once finished. */
export function stepAt(tl: Timeline, t: number): { step: TimedStep; index: number; progress: number } | null {
  const at = Math.max(0, t);
  for (const [index, s] of tl.steps.entries()) {
    if (at < s.start + s.hold) return { step: s, index, progress: (at - s.start) / s.hold };
  }
  return null;
}

const POINT: Record<string, { x: number; y: number }> = {
  up: { x: 0, y: -1 }, down: { x: 0, y: 1 }, left: { x: -1, y: 0 }, right: { x: 1, y: 0 },
  up_left: { x: -1, y: -1 }, up_right: { x: 1, y: -1 }, down_left: { x: -1, y: 1 }, down_right: { x: 1, y: 1 },
};

/** Where the guide should be (x, y in -1..1, y down) for a direction at progress p (0..1). */
export function positionAt(direction: string, p: number): { x: number; y: number } {
  const a = 2 * Math.PI * p;
  switch (direction) {
    case "circle_cw":
      return { x: Math.sin(a), y: -Math.cos(a) };
    case "circle_ccw":
      return { x: -Math.sin(a), y: -Math.cos(a) };
    case "figure8":
      return { x: Math.sin(a), y: Math.sin(2 * a) / 2 };
  }
  return POINT[direction] ?? { x: 0, y: 0 };
}
