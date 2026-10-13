// audit: domain model (types and pure rules) — no I/O.
export type AuditEntry = { at: string; actor_id: string; actor_name: string; action: string; target: string; status: number };
