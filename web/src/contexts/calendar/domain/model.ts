// calendar: domain model (types and pure rules) — no I/O.
export type Scope = "people" | "project" | "workspace";
export type MeetingResponse = "going" | "maybe" | "declined";
export type Meeting = {
  id: string;
  title: string;
  starts_at: string;
  ends_at: string;
  organizer_id: string;
  attendee_ids: string[];
  scope?: Scope;
  project_key?: string | null;
  /** Attendee id → answer, for those who answered. */
  responses?: Record<string, MeetingResponse>;
};
export type MeetingInput = {
  title: string;
  starts_at: string;
  ends_at: string;
  attendee_ids: string[];
  scope?: Scope;
  project_key?: string;
};

export const RESPONSES: { id: MeetingResponse; label: string }[] = [
  { id: "going", label: "Going" },
  { id: "maybe", label: "Maybe" },
  { id: "declined", label: "Decline" },
];

/** tally counts the answers, e.g. "2 going · 1 maybe" ("" when nobody answered). */
export function tally(m: Pick<Meeting, "responses">): string {
  const counts = { going: 0, maybe: 0, declined: 0 };
  for (const r of Object.values(m.responses ?? {})) counts[r]++;
  return (["going", "maybe", "declined"] as const)
    .filter((k) => counts[k] > 0)
    .map((k) => `${counts[k]} ${k}`)
    .join(" · ");
}

/** Who a meeting is for, in a word: a project key, "Everyone", or "" for chosen people. */
export function audienceLabel(m: Pick<Meeting, "scope" | "project_key">): string {
  if (m.scope === "project" && m.project_key) return m.project_key;
  return m.scope === "workspace" ? "Everyone" : "";
}
