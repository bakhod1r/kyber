// importer: domain model (types and pure rules) — no I/O.
import type { IssueType, Priority, Status } from "../../issue/domain/model";

export type ImportItem = {
  line: number;
  external_key: string;
  issue_key: string | null;
  title: string;
  type: IssueType;
  status: Status;
  priority: Priority;
  assignee_id: string | null;
  points: number | null;
  created_at: string | null;
  resolved_at: string | null;
  warnings: string[];
};
export type ImportReport = {
  dry_run: boolean;
  items: ImportItem[];
  errors: { line: number; external_key: string; message: string }[];
  skipped: string[];
};
