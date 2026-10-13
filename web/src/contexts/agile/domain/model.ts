// agile: domain model (types and pure rules) — no I/O.
import type { Status } from "../../issue/domain/model";

export type SprintState = "planned" | "active" | "closed";
export type Sprint = {
  id: string;
  project_key: string;
  name: string;
  goal: string;
  state: SprintState;
  started_at: string | null;
  completed_at: string | null;
};
export type Completion = { sprint: Sprint; completed: number; returned: number };
export type VersionState = "unreleased" | "released" | "archived";
export type Version = {
  id: string;
  project_key: string;
  name: string;
  description: string;
  release_date: string | null;
  state: VersionState;
  released_at: string | null;
  version: number;
  issues?: number;
  done?: number;
  unresolved?: number;
};
export type VersionInput = { name: string; description: string; release_date: string | null };
export type BoardColumn = { name: string; statuses: Status[]; min: number; max: number };
export type BoardView = { id: string; project_key: string; name: string; kind: "scrum" | "kanban"; columns: BoardColumn[]; version: number; kanban_backlog?: boolean };

// ---- board configuration rules (pure)

/** Moves column i by delta (-1 up, +1 down); out-of-range moves change nothing. */
export function moveColumn(cols: BoardColumn[], i: number, delta: number): BoardColumn[] {
  const j = i + delta;
  if (i < 0 || i >= cols.length || j < 0 || j >= cols.length) return cols;
  const out = [...cols];
  [out[i], out[j]] = [out[j] as BoardColumn, out[i] as BoardColumn];
  return out;
}

/** Puts a status in column `to` (-1 = off the board); a status is in at most one column. */
export function assignStatus(cols: BoardColumn[], status: Status, to: number): BoardColumn[] {
  return cols.map((c, i) => {
    const rest = c.statuses.filter((s) => s !== status);
    return { ...c, statuses: i === to ? [...rest, status] : rest };
  });
}

/** The column showing a status, or -1. */
export function columnOf(cols: BoardColumn[], status: Status): number {
  return cols.findIndex((c) => c.statuses.includes(status));
}

/** Adds an empty column with a name not used yet ("New column", "New column 2", ...). */
export function addColumn(cols: BoardColumn[]): BoardColumn[] {
  const taken = new Set(cols.map((c) => c.name.toLowerCase()));
  let name = "New column";
  for (let n = 2; taken.has(name.toLowerCase()); n++) name = `New column ${n}`;
  return [...cols, { name, statuses: [], min: 0, max: 0 }];
}

/** Problems that would make the server refuse the board (shown before saving). */
export function boardProblems(name: string, cols: BoardColumn[]): string[] {
  const out: string[] = [];
  if (!name.trim()) out.push("The board needs a name.");
  if (cols.length === 0) out.push("A board needs at least one column.");
  const seen = new Set<string>();
  for (const c of cols) {
    const n = c.name.trim().toLowerCase();
    if (!n) out.push("Every column needs a name.");
    else if (seen.has(n)) out.push(`Two columns are called "${c.name.trim()}".`);
    seen.add(n);
    if (c.max > 0 && c.min > c.max) out.push(`"${c.name}": the minimum is above the maximum.`);
  }
  return out;
}
