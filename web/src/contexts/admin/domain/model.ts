// admin: domain model — what a workspace's administrators see.
export type Role = "owner" | "admin" | "member";
export type AdminMember = { id: string; name: string; email: string; role: Role };
export type AdminProject = { key: string; name: string; members: number };
export type DeadLetter = { id: number; event: string; actor_id: string; at: string; attempts: number; last_error: string };
export const ROLES: Role[] = ["owner", "admin", "member"];
