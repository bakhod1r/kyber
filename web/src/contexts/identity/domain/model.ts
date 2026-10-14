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

/** safeNext keeps a post-login return path on this site (no absolute or protocol-relative URLs). */
export function safeNext(raw: string | null): string {
  return raw && raw.startsWith("/") && !raw.startsWith("//") && !raw.startsWith("/\\") ? raw : "/";
}

/** An MCP client's OAuth authorization request (ADR-0016), as the consent screen approves it. */
export type AuthorizeRequest = { client_id: string; redirect_uri: string; code_challenge: string; code_challenge_method: string; state?: string };
export type OAuthClient = { client_id: string; client_name: string; redirect_uris: string[] };

/** authorizeRequest reads the query of /oauth/authorize; null when it is not a code + PKCE request. */
export function authorizeRequest(q: URLSearchParams): AuthorizeRequest | null {
  const client_id = q.get("client_id"), redirect_uri = q.get("redirect_uri"), code_challenge = q.get("code_challenge");
  if (q.get("response_type") !== "code" || !client_id || !redirect_uri || !code_challenge) return null;
  const state = q.get("state");
  return { client_id, redirect_uri, code_challenge, code_challenge_method: q.get("code_challenge_method") ?? "S256", ...(state ? { state } : {}) };
}

/** registeredRedirect mirrors the server: exact match, or a loopback redirect on another port. */
export function registeredRedirect(client: OAuthClient, redirect: string): boolean {
  if (client.redirect_uris.includes(redirect)) return true;
  try {
    const u = new URL(redirect);
    const loop = ["localhost", "127.0.0.1", "[::1]"];
    return u.protocol === "http:" && loop.includes(u.hostname) && client.redirect_uris.some((r) => {
      const v = new URL(r);
      return v.protocol === "http:" && v.hostname === u.hostname && v.pathname === u.pathname && v.search === u.search;
    });
  } catch {
    return false;
  }
}
