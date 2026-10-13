// project: domain model (types and pure rules) — no I/O.
export type Project = { id: string; key: string; name: string };
export type Role = "admin" | "member" | "viewer";
export type Member = { user_id: string; email: string; name: string; role: Role };
