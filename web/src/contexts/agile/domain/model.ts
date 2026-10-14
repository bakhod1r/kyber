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

/** Swimlanes split the board into horizontal rows (Jira): by assignee, by epic, or none. */
export type SwimlaneMode = "none" | "assignee" | "epic";
export const SWIMLANE_MODES: SwimlaneMode[] = ["none", "assignee", "epic"];

/** A lane: `id` is the assignee id or epic key ("" for issues without one). */
export type Lane<T> = { id: string; items: T[] };

/** swimlanes groups issues into lanes, keeping their order; lanes come in order of first
 * appearance and the lane without an assignee/epic comes last. Mode "none" gives one lane. */
export function swimlanes<T extends { assignee_id: string | null; parent_key?: string | null }>(issues: T[], mode: SwimlaneMode): Lane<T>[] {
  if (mode === "none") return [{ id: "", items: issues }];
  const lanes = new Map<string, T[]>();
  for (const i of issues) {
    const id = (mode === "assignee" ? i.assignee_id : i.parent_key) ?? "";
    lanes.set(id, [...(lanes.get(id) ?? []), i]);
  }
  const out = [...lanes].map(([id, items]) => ({ id, items }));
  return [...out.filter((l) => l.id !== ""), ...out.filter((l) => l.id === "")];
}

/** The board toolbar's filters (Jira): text, assignees ("" = unassigned), fix versions, epics
 * ("" = no epic), types and labels. Empty lists do not filter. */
export type BoardFilter = { text: string; assignees: string[]; versions: string[]; epics: string[]; types: string[]; labels: string[] };
export const NO_BOARD_FILTER: BoardFilter = { text: "", assignees: [], versions: [], epics: [], types: [], labels: [] };

export function boardFilterActive(f: BoardFilter): boolean {
  return f.text.trim() !== "" || f.assignees.length + f.versions.length + f.epics.length + f.types.length + f.labels.length > 0;
}

type Filterable = { key: string; title: string; type: string; assignee_id: string | null; parent_key?: string | null; fix_versions?: string[]; labels?: string[] };

/** filterBoard keeps the issues matching every active filter; text matches key or title. */
export function filterBoard<T extends Filterable>(issues: T[], f: BoardFilter): T[] {
  const text = f.text.trim().toLowerCase();
  return issues.filter(
    (i) =>
      (!text || i.key.toLowerCase().includes(text) || i.title.toLowerCase().includes(text)) &&
      (f.assignees.length === 0 || f.assignees.includes(i.assignee_id ?? "")) &&
      (f.versions.length === 0 || (i.fix_versions ?? []).some((v) => f.versions.includes(v))) &&
      (f.epics.length === 0 || f.epics.includes(i.parent_key ?? "")) &&
      (f.types.length === 0 || f.types.includes(i.type)) &&
      (f.labels.length === 0 || (i.labels ?? []).some((l) => f.labels.includes(l))),
  );
}

/** toggle adds or removes one value of a multi-select filter. */
export function toggle(list: string[], value: string): string[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}
