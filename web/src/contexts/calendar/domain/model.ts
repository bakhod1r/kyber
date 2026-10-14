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

/** A meeting's new title and time (organizer only). */
export type MeetingChange = Pick<MeetingInput, "title" | "starts_at" | "ends_at">;

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

type Parts = Record<"weekday" | "day" | "month" | "year" | "hour" | "minute", string>;

function parts(iso: string, zone: string): Parts {
  const out: Record<string, string> = {};
  const f = new Intl.DateTimeFormat("en-GB", {
    timeZone: zone, weekday: "short", day: "2-digit", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit", hourCycle: "h23",
  });
  for (const p of f.formatToParts(new Date(iso))) out[p.type] = p.value;
  const month = new Intl.DateTimeFormat("en-GB", { timeZone: zone, month: "2-digit" }).format(new Date(iso));
  return { ...(out as Parts), year: `${out.year}-${month}` };
}

/** when shows a meeting's time in a time zone (an IANA name), naming it: "Sat, 17 Oct, 20:00 – 21:30 (Asia/Tashkent)". */
export function when(m: Pick<Meeting, "starts_at" | "ends_at">, zone: string): string {
  const s = parts(m.starts_at, zone);
  const e = parts(m.ends_at, zone);
  return `${s.weekday}, ${Number(s.day)} ${s.month}, ${s.hour}:${s.minute} – ${e.hour}:${e.minute} (${zone})`;
}

/** localInput is the datetime-local value (YYYY-MM-DDTHH:mm) of an instant in a time zone. */
export function localInput(iso: string, zone: string): string {
  const p = parts(iso, zone);
  return `${p.year}-${p.day}T${p.hour}:${p.minute}`;
}

/** minutesOf is a meeting's length in minutes. */
export function minutesOf(m: Pick<Meeting, "starts_at" | "ends_at">): number {
  return Math.round((Date.parse(m.ends_at) - Date.parse(m.starts_at)) / 60_000);
}

/** Who a meeting is for, in a word: a project key, "Everyone", or "" for chosen people. */
export function audienceLabel(m: Pick<Meeting, "scope" | "project_key">): string {
  if (m.scope === "project" && m.project_key) return m.project_key;
  return m.scope === "workspace" ? "Everyone" : "";
}
