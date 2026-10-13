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
