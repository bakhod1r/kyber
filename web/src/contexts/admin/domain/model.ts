// admin: domain model — what a workspace's administrators see.
export type Role = "owner" | "admin" | "member";
export type AdminMember = { id: string; name: string; email: string; role: Role };
export type AdminProject = { key: string; name: string; members: number };
export type DeadLetter = { id: number; event: string; actor_id: string; at: string; attempts: number; last_error: string };
export const ROLES: Role[] = ["owner", "admin", "member"];
export type MappingScope = "workspace" | "project";
export type GroupMapping = { id: string; group: string; scope: MappingScope; workspace_id: string; project_key: string; role: string; created_at: string };
export type NewGroupMapping = { group: string; scope: MappingScope; project_key?: string; role: string };
export const MAPPED_ROLES: Record<MappingScope, string[]> = { workspace: ["member", "admin"], project: ["viewer", "member", "admin"] };
export type SSODomain = { name: string; verified: boolean; txt: string };
export type SSOConnection = { id: string; kind: "oidc"; name: string; issuer: string; client_id: string; groups_claim: string; workspace_id?: string | null; enforced?: boolean; enabled: boolean; created_at: string; domains: SSODomain[] };
export type ConnectionScope = "workspace" | "installation";
export type NewSSOConnection = { name: string; issuer: string; client_id: string; client_secret: string; groups_claim: string; scope: ConnectionScope };
