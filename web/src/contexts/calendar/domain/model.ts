// calendar: domain model (types and pure rules) — no I/O.
export type Meeting = { id: string; title: string; starts_at: string; ends_at: string; organizer_id: string; attendee_ids: string[] };
