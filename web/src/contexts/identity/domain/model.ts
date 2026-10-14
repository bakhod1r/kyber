// identity: domain model (types and pure rules) — no I/O.
// Typed client for the Kyber REST API (v1). Cookie session; JSON content type on
// every request satisfies the server's CSRF guard.

export type User = { id: string; email: string; name: string };
export type AuthProviders = { google: boolean; oidc?: string | null; github?: boolean; telegram_bot: string | null; telegram_otp?: boolean };
export type OtpChallenge = { id: string; link: string; expires_at: string };
/** The Telegram Login Widget's user object, forwarded unchanged for server-side verification. */
export type TelegramUser = { id: number; auth_date: number; hash: string; first_name?: string; last_name?: string; username?: string; photo_url?: string };

/** PersonalToken: a long-lived credential for scripts and AI assistants (MCP); the secret is shown once. */
export type PersonalToken = { id: string; name: string; created_at: string; expires_at: string | null; last_used_at: string | null };
export type CreatedToken = PersonalToken & { token: string };
