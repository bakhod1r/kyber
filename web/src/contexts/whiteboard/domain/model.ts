// whiteboard: domain model — Zoom-style boards drawn together (S59). Pure types and rules, no I/O.
export type Kind = "rect" | "ellipse" | "arrow" | "line" | "freehand" | "text" | "sticky";
export type Point = { x: number; y: number };
export type Element = {
  id: string; kind: Kind; x: number; y: number; w: number; h: number; points: Point[]; text: string; color: string;
  version: number; updated_by?: string; updated_at?: string; deleted?: boolean;
};
export type Whiteboard = { id: string; conversation_id: string; title: string; created_by: string; created_at: string; elements?: Element[] };
export type OpKind = "add" | "update" | "delete";
export type Op = { op: OpKind; element: Element };
export type OpResult = { applied: boolean; error: "" | "stale" | "not_found" | "exists" | "full" | "invalid"; element: Element };

export type Elements = Record<string, Element>;

export const COLORS = ["#1f2937", "#0b6fd8", "#16a34a", "#dc2626", "#f59e0b", "#9333ea"];
export const STICKY = "#fde68a";

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
