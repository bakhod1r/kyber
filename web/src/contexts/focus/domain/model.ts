// focus: domain model (types and pure rules) — no I/O.
export type FocusState = "running" | "paused" | "completed" | "interrupted";
export type FocusSession = {
  id: string;
  issue_key: string;
  user_id: string;
  state: FocusState;
  planned_minutes: number;
  started_at: string;
  ended_at: string | null;
  focused_seconds: number;
  remaining_seconds: number;
  /** Exact time left (ms); absolute deadline while running; server clock at response time. */
  remaining_ms?: number;
  ends_at?: string | null;
  server_time?: string;
  reason: string;
};
