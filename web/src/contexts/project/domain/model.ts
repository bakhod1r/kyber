// project: domain model (types and pure rules) — no I/O.
export type Project = { id: string; key: string; name: string };
export type Role = "admin" | "member" | "viewer";
export type Member = { user_id: string; email: string; name: string; role: Role };
/** Invitation: someone asked by email to join a project (B1); the link works until expires_at. */
export type Invitation = { id: string; email: string; role: Role; invited_by: string; created_at: string; expires_at: string };
/** InviteResult: an existing user is added at once; anyone else gets an invitation (emailed, or a link to share). */
export type InviteResult = { added: true } | { added: false; invitation: Invitation; link: string; emailed: boolean };
/** InvitationPreview: what the holder of a link sees before accepting. */
export type InvitationPreview = { project: string; project_name: string; invited_by: string; email: string; role: Role; expires_at: string; expired: boolean };
