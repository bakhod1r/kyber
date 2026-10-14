// whiteboard: domain model — Zoom-style boards drawn together (S59). Pure types and rules, no I/O.
export type Kind = "rect" | "ellipse" | "arrow" | "line" | "freehand" | "text" | "sticky";
export type Point = { x: number; y: number };
export type Element = {
  id: string; kind: Kind; x: number; y: number; w: number; h: number; points: Point[]; text: string; color: string;
  /** S91 styles: 1 thin, 2 medium, 3 bold (absent = default); filled shapes; dashed lines. */
  stroke?: 0 | 1 | 2 | 3; fill?: boolean; dashed?: boolean;
  version: number; updated_by?: string; updated_at?: string; deleted?: boolean;
};
export type Whiteboard = { id: string; conversation_id: string; title: string; created_by: string; created_at: string; elements?: Element[] };
export type OpKind = "add" | "update" | "delete";
export type Op = { op: OpKind; element: Element };
export type OpResult = { applied: boolean; error: "" | "stale" | "not_found" | "exists" | "full" | "invalid"; element: Element };

export type Elements = Record<string, Element>;

export const COLORS = [
  "#1f2937", "#64748b", "#0b6fd8", "#0891b2", "#16a34a", "#65a30d", "#f59e0b", "#ea580c", "#dc2626", "#db2777", "#9333ea", "#7c3aed",
];
export const STICKY = "#fde68a";
/** S91: sticky note colours (yellow first, the default). */
export const STICKY_COLORS = [STICKY, "#fbcfe8", "#bbf7d0", "#bfdbfe", "#fed7aa", "#e9d5ff"];

/** A fresh id for a new element (unique enough within one board). */
export const newId = () => Math.random().toString(36).slice(2, 10) + Date.now().toString(36).slice(-4);

/** Applies a change locally (optimistically, before the server confirms). */
export function applyLocal(els: Elements, op: Op): Elements {
  const next = { ...els };
  if (op.op === "delete") delete next[op.element.id];
  else next[op.element.id] = op.element;
  return next;
}

/** Takes the server's word for elements: what it confirmed, or the current state of what it refused. */
export function reconcile(els: Elements, results: Pick<OpResult, "element">[]): Elements {
  const next = { ...els };
  for (const { element } of results) {
    if (element.deleted) delete next[element.id];
    else next[element.id] = element;
  }
  return next;
}

/** The elements of an event that are newer than what we already have (by element version). */
export const newer = (known: Elements, incoming: Element[]) => incoming.filter((e) => (known[e.id]?.version ?? 0) < e.version);

/** The change that undoes op, given the element before it (if any). */
export function inverse(op: Op, before: Element | undefined): Op {
  if (op.op === "add") return { op: "delete", element: op.element };
  if (op.op === "delete") return { op: "add", element: { ...(before ?? op.element), version: 0 } };
  return { op: "update", element: before ?? op.element };
}

/** Normalises a box drawn from (x0,y0) to (x1,y1) in any direction. */
export function box(x0: number, y0: number, x1: number, y1: number) {
  return { x: Math.min(x0, x1), y: Math.min(y0, y1), w: Math.abs(x1 - x0), h: Math.abs(y1 - y0) };
}

/** Whether a point hits an element (boxes by area; strokes near their points). */
export function hit(e: Element, p: Point, slack = 6): boolean {
  if (e.kind === "line" || e.kind === "arrow" || e.kind === "freehand") {
    const pts = e.points.map((q) => ({ x: e.x + q.x, y: e.y + q.y }));
    for (let i = 1; i < pts.length; i++) if (distToSegment(p, pts[i - 1]!, pts[i]!) <= slack) return true;
    return false;
  }
  return p.x >= e.x - slack && p.x <= e.x + e.w + slack && p.y >= e.y - slack && p.y <= e.y + e.h + slack;
}

function distToSegment(p: Point, a: Point, b: Point) {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const len = dx * dx + dy * dy;
  const t = len === 0 ? 0 : Math.max(0, Math.min(1, ((p.x - a.x) * dx + (p.y - a.y) * dy) / len));
  return Math.hypot(p.x - (a.x + t * dx), p.y - (a.y + t * dy));
}

/** Drawing order: oldest change first (newer shapes on top). */
export const ordered = (els: Elements) => Object.values(els).sort((a, b) => (a.updated_at ?? "~").localeCompare(b.updated_at ?? "~"));

/** A sticky note as an issue: the first line is the title, the rest the description (S59). */
export function stickyToIssue(text: string): { title: string; description: string } {
  const [first = "", ...rest] = text.trim().split("\n");
  return { title: first.trim().slice(0, 255), description: rest.join("\n").trim() };
}

/** The issue a sticky note was turned into ("→ KYB-12" on its last line), if any. */
export function linkedIssue(text: string): string | undefined {
  return /(?:^|\n)→ ([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)\s*$/.exec(text)?.[1];
}

/** A comment pinned to an element (S86). */
export type ElementComment = { id: string; element_id: string; author_id: string; body: string; created_at: string };

/** How many comments each element has. */
export function commentCounts(comments: ElementComment[]): Map<string, number> {
  const out = new Map<string, number>();
  for (const c of comments) out.set(c.element_id, (out.get(c.element_id) ?? 0) + 1);
  return out;
}

/** S89: what part of the infinite board is shown — the top-left corner (board units) and the zoom (1 = 100 %). */
export type View = { x: number; y: number; zoom: number };
export const HOME: View = { x: 0, y: 0, zoom: 1 };
export const MIN_ZOOM = 0.1;
export const MAX_ZOOM = 4;
const clampZoom = (z: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, z));

/** zoomAt scales the view by factor, keeping the board point p under the pointer where it is. */
export function zoomAt(v: View, factor: number, p: Point): View {
  const zoom = clampZoom(v.zoom * factor);
  const k = v.zoom / zoom;
  return { x: p.x - (p.x - v.x) * k, y: p.y - (p.y - v.y) * k, zoom };
}

/** panBy moves the view by a distance dragged on screen (in page units: the board drags along). */
export const panBy = (v: View, dx: number, dy: number): View => ({ ...v, x: v.x - dx / v.zoom, y: v.y - dy / v.zoom });

/** viewBox is the SVG viewBox showing v on a page of the given size. */
export const viewBox = (v: View, page: { w: number; h: number }) => `${v.x} ${v.y} ${page.w / v.zoom} ${page.h / v.zoom}`;

/** onScreen places a board point on the page, in percent (for HTML laid over the canvas). */
export const onScreen = (v: View, page: { w: number; h: number }, p: Point) => ({
  left: (((p.x - v.x) * v.zoom) / page.w) * 100,
  top: (((p.y - v.y) * v.zoom) / page.h) * 100,
});

/** bounds is the box around every element (strokes by their points); undefined for an empty board. */
export function bounds(els: Element[]): { x: number; y: number; w: number; h: number } | undefined {
  if (els.length === 0) return undefined;
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
  for (const e of els) {
    const pts = e.points.length > 0 ? e.points.map((q) => ({ x: e.x + q.x, y: e.y + q.y })) : [{ x: e.x, y: e.y }, { x: e.x + e.w, y: e.y + e.h }];
    for (const p of pts) {
      x0 = Math.min(x0, p.x);
      y0 = Math.min(y0, p.y);
      x1 = Math.max(x1, p.x);
      y1 = Math.max(y1, p.y);
    }
  }
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

/** fit shows every element with a margin, centred, never zoomed in past 400 %. */
export function fit(els: Element[], page: { w: number; h: number }, margin = 40): View {
  const b = bounds(els);
  if (!b) return HOME;
  const zoom = clampZoom(Math.min(page.w / (b.w + 2 * margin), page.h / (b.h + 2 * margin)));
  return { x: b.x + b.w / 2 - page.w / zoom / 2, y: b.y + b.h / 2 - page.h / zoom / 2, zoom };
}

/** S90: the eight handles that resize a box. */
export type Handle = "n" | "s" | "e" | "w" | "ne" | "nw" | "se" | "sw";
export const HANDLES: Handle[] = ["nw", "n", "ne", "e", "se", "s", "sw", "w"];
export const MIN_SIZE = 8;

/** resize drags handle h of a box by (dx, dy); the opposite side stays put and the box never turns inside out. */
export function resize(e: Element, h: Handle, dx: number, dy: number): Element {
  let { x, y, w, h: height } = e;
  if (h.includes("e")) w = Math.max(MIN_SIZE, w + dx);
  if (h.includes("s")) height = Math.max(MIN_SIZE, height + dy);
  if (h.includes("w")) {
    const right = x + w;
    w = Math.max(MIN_SIZE, w - dx);
    x = right - w;
  }
  if (h.includes("n")) {
    const bottom = y + height;
    height = Math.max(MIN_SIZE, height - dy);
    y = bottom - height;
  }
  return { ...e, x, y, w, h: height };
}

/** moveEnd puts point i of a line or arrow at board point p. */
export function moveEnd(e: Element, i: number, p: Point): Element {
  return { ...e, points: e.points.map((q, j) => (j === i ? { x: p.x - e.x, y: p.y - e.y } : q)) };
}

/** moveBy shifts every element by (dx, dy). */
export const moveBy = (els: Element[], dx: number, dy: number): Element[] => els.map((e) => ({ ...e, x: e.x + dx, y: e.y + dy }));

/** within lists the elements a marquee covers completely. */
export function within(els: Element[], area: { x: number; y: number; w: number; h: number }): Element[] {
  return els.filter((e) => {
    const b = bounds([e])!;
    return b.x >= area.x && b.y >= area.y && b.x + b.w <= area.x + area.w && b.y + b.h <= area.y + area.h;
  });
}

/** duplicate copies elements under fresh ids, offset so the copy is visible, as new (version 0). */
export const duplicate = (els: Element[], offset: number, id: () => string = newId): Element[] =>
  els.map(({ updated_at: _at, updated_by: _by, ...e }) => ({ ...e, id: id(), x: e.x + offset, y: e.y + offset, version: 0 }));

/** toClipboard / fromClipboard carry elements as text, so they paste across boards and tabs. */
export const toClipboard = (els: Element[]) => JSON.stringify({ kyber: "whiteboard", elements: els });
export function fromClipboard(text: string): Element[] {
  try {
    const v = JSON.parse(text) as { kyber?: unknown; elements?: unknown };
    if (v.kyber !== "whiteboard" || !Array.isArray(v.elements)) return [];
    const ok = (e: unknown): e is Element =>
      typeof e === "object" && e !== null && typeof (e as Element).id === "string" && typeof (e as Element).kind === "string" &&
      typeof (e as Element).x === "number" && typeof (e as Element).y === "number" && Array.isArray((e as Element).points);
    return v.elements.every(ok) ? v.elements : [];
  } catch {
    return [];
  }
}

/** inverseAll undoes a batch of changes: each one inverted, last first. */
export const inverseAll = (ops: Op[], before: Elements): Op[] => ops.map((op) => inverse(op, before[op.element.id])).reverse();

/** S91: how an element's style is drawn — stroke width, a light fill tint of its colour, dashes. */
export function look(e: Pick<Element, "kind" | "color" | "stroke" | "fill" | "dashed">) {
  const strokeWidth = ({ 0: 2, 1: 1, 2: 3, 3: 6 } as const)[e.stroke ?? 0];
  const fillable = e.kind === "rect" || e.kind === "ellipse";
  return { strokeWidth, fill: fillable && e.fill ? `${e.color || "#1f2937"}33` : "none", dash: e.dashed ? "10 6" : undefined };
}

/** restyle applies a style to the elements it means something for (sticky notes and text have no stroke). */
export function restyle(els: Element[], style: Pick<Element, "stroke" | "fill" | "dashed">): Element[] {
  return els
    .filter((e) => e.kind !== "sticky" && e.kind !== "text")
    .map((e) => {
      const fillable = e.kind === "rect" || e.kind === "ellipse";
      const next = { ...e, ...style };
      if (!fillable) delete next.fill;
      return next;
    });
}

/** S92: boards to start from. */
export type Template = "retro" | "kanban" | "swot";
export const TEMPLATES: { id: Template; label: string }[] = [
  { id: "retro", label: "Retrospective" },
  { id: "kanban", label: "Kanban" },
  { id: "swot", label: "SWOT" },
];

/** template lays a template out as ordinary new elements (columns or quadrants with a heading), centred on c. */
export function template(kind: Template, c: Point, id: () => string = newId): Element[] {
  const el = (e: Pick<Element, "kind" | "x" | "y" | "w" | "h"> & Partial<Element>): Element => ({ points: [], text: "", color: "#1f2937", version: 0, ...e, id: id() });
  const panels: { title: string; x: number; y: number; w: number; h: number; color: string }[] = [];
  if (kind === "swot") {
    const titles: [string, string][] = [["Strengths", "#16a34a"], ["Weaknesses", "#dc2626"], ["Opportunities", "#0b6fd8"], ["Threats", "#f59e0b"]];
    titles.forEach(([title, color], i) => panels.push({ title, color, x: (i % 2) * 520, y: Math.floor(i / 2) * 380, w: 500, h: 360 }));
  } else {
    const titles: [string, string][] =
      kind === "retro"
        ? [["Went well", "#16a34a"], ["To improve", "#f59e0b"], ["Actions", "#0b6fd8"]]
        : [["To do", "#64748b"], ["Doing", "#0b6fd8"], ["Done", "#16a34a"]];
    titles.forEach(([title, color], i) => panels.push({ title, color, x: i * 420, y: 0, w: 400, h: 640 }));
  }
  const w = Math.max(...panels.map((p) => p.x + p.w));
  const h = Math.max(...panels.map((p) => p.y + p.h));
  const ox = c.x - w / 2;
  const oy = c.y - h / 2;
  return panels.flatMap((p) => [
    el({ kind: "rect", x: ox + p.x, y: oy + p.y, w: p.w, h: p.h, color: p.color, fill: true }),
    el({ kind: "text", x: ox + p.x + 12, y: oy + p.y + 10, w: p.w - 24, h: 34, text: p.title, color: p.color }),
  ]);
}
