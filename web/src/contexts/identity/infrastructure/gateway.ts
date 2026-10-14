// identity: HTTP gateway to the identity API (infrastructure).
import { request } from "../../../shared/http";
import type { AuthorizeRequest, AuthProviders, CreatedToken, OAuthClient, OtpChallenge, PersonalToken, TelegramUser, User } from "../domain/model";

export const identityGateway = {
  me: () => request<User>("GET", "/api/v1/me"),
  signup: (email: string, name: string, password: string) =>
    request<User>("POST", "/api/v1/auth/signup", { email, name, password }),
  login: (email: string, password: string) => request<{ token: string }>("POST", "/api/v1/auth/login", { email, password }),
  authProviders: () => request<AuthProviders>("GET", "/api/v1/auth/providers"),
  telegramOtpStart: () => request<OtpChallenge>("POST", "/api/v1/auth/telegram/otp", {}),
  telegramOtpVerify: (id: string, code: string) => request<{ token: string }>("POST", "/api/v1/auth/telegram/otp/verify", { id, code }),
  telegramLogin: (user: TelegramUser) => request<{ token: string }>("POST", "/api/v1/auth/telegram", user),
  logout: () => request<undefined>("POST", "/api/v1/auth/logout"),

  tokens: () => request<{ items: PersonalToken[] }>("GET", "/api/v1/me/tokens").then((r) => r.items),
  createToken: (name: string, expiresInDays: number) => request<CreatedToken>("POST", "/api/v1/me/tokens", { name, expires_in_days: expiresInDays }),
  revokeToken: (id: string) => request<undefined>("DELETE", `/api/v1/me/tokens/${encodeURIComponent(id)}`),
  oauthClient: (id: string) => request<OAuthClient>("GET", `/api/v1/oauth/clients/${encodeURIComponent(id)}`),
  approveOAuth: (r: AuthorizeRequest) => request<{ redirect_to: string }>("POST", "/api/v1/oauth/authorize", r),
  connectTelegram: () => request<{ url: string; expires_at: string }>("POST", "/api/v1/me/telegram/connect"),
};
