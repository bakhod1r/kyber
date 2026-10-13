// identity: HTTP gateway to the identity API (infrastructure).
import { request } from "../../../shared/http";
import type { AuthProviders, OtpChallenge, TelegramUser, User } from "../domain/model";

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

  connectTelegram: () => request<{ url: string; expires_at: string }>("POST", "/api/v1/me/telegram/connect"),
};
