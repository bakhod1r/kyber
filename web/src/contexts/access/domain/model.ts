// access: domain model (types and pure rules) — no I/O.
export type GrantKind = "role" | "user" | "reporter" | "assignee";
export type PermissionGrants = { permission: string; grants: { kind: GrantKind; value: string }[] };
export type PermissionScheme = { custom: boolean; permissions: PermissionGrants[] };
