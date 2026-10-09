import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { AppRoutes } from "../App";
import { mockApi, renderWithProviders } from "../test-utils";

const anonymous = { status: 401, body: { type: "about:blank", title: "Please log in.", status: 401, code: "AUTH_REQUIRED" } };
const tgUser = { id: 42, first_name: "Bobur", username: "bobur", auth_date: 1760000000, hash: "abc" };

describe("Sign in with Google and Telegram (KYB-S39)", () => {
  it("AC1: shows the configured providers", async () => {
    mockApi({
      "GET /api/v1/me": anonymous,
      "GET /api/v1/auth/providers": { status: 200, body: { google: true, telegram_bot: "kyber_bot" } },
    });
    renderWithProviders(<AppRoutes />, "/login");
    expect(await screen.findByRole("link", { name: "Continue with Google" })).toHaveAttribute("href", "/api/v1/auth/google/start");
    const widget = await waitFor(() => {
      const s = document.querySelector<HTMLScriptElement>('script[data-telegram-login="kyber_bot"]');
      if (!s) throw new Error("no widget");
      return s;
    });
    expect(widget.src).toContain("telegram.org/js/telegram-widget.js");
    expect(widget.dataset.onauth).toBe("kyberTelegramAuth(user)");
  });

  it("AC2: the Telegram widget callback signs in and opens the projects", async () => {
    let signedIn = false;
    const calls = mockApi({
      "GET /api/v1/me": () => (signedIn ? { status: 200, body: { id: "u-1", email: "telegram-42@users.kyber.invalid", name: "Bobur" } } : anonymous),
      "GET /api/v1/auth/providers": { status: 200, body: { google: false, telegram_bot: "kyber_bot" } },
      "POST /api/v1/auth/telegram": () => {
        signedIn = true;
        return { status: 200, body: { token: "t" } };
      },
      "GET /api/v1/projects": { status: 200, body: { items: [] } },
    });
    renderWithProviders(<AppRoutes />, "/login");
    await waitFor(() => expect(window.kyberTelegramAuth).toBeTypeOf("function"));
    await act(async () => window.kyberTelegramAuth?.(tgUser));
    expect(await screen.findByText(/No projects yet/)).toBeInTheDocument();
    expect(calls.find((c) => c.key === "POST /api/v1/auth/telegram")?.body).toEqual(tgUser);
    expect(screen.queryByRole("link", { name: "Continue with Google" })).not.toBeInTheDocument();
  });

  it("AC3: explains why a social sign-in failed", async () => {
    mockApi({ "GET /api/v1/me": anonymous, "GET /api/v1/auth/providers": { status: 200, body: { google: true, telegram_bot: null } } });
    renderWithProviders(<AppRoutes />, "/login?error=google_email_taken");
    expect(await screen.findByRole("alert")).toHaveTextContent("already has a Kyber account");
  });

  it("AC4: nothing extra when no provider is configured", async () => {
    mockApi({ "GET /api/v1/me": anonymous, "GET /api/v1/auth/providers": { status: 200, body: { google: false, telegram_bot: null } } });
    renderWithProviders(<AppRoutes />, "/login");
    await screen.findByRole("heading", { name: "Log in to Kyber" });
    expect(screen.queryByText(/or continue with/i)).not.toBeInTheDocument();
  });

  it("S40: logs in with a code sent by the Telegram bot", async () => {
    let signedIn = false;
    const calls = mockApi({
      "GET /api/v1/me": () => (signedIn ? { status: 200, body: { id: "u-1", email: "t@x", name: "Dilnoza" } } : anonymous),
      "GET /api/v1/auth/providers": { status: 200, body: { google: false, telegram_bot: null, telegram_otp: true } },
      "POST /api/v1/auth/telegram/otp": { status: 201, body: { id: "c-1", link: "https://t.me/kyber_bot?start=abc", expires_at: "2026-10-09T12:05:00Z" } },
      "POST /api/v1/auth/telegram/otp/verify": (init) => {
        const body = JSON.parse(String(init?.body)) as { code: string };
        if (body.code !== "123456") {
          return { status: 401, body: { type: "about:blank", title: "x", status: 401, code: "AUTH_OTP_INVALID", detail: "wrong login code" } };
        }
        signedIn = true;
        return { status: 200, body: { token: "t" } };
      },
      "GET /api/v1/projects": { status: 200, body: { items: [] } },
    });
    const user = userEvent.setup();
    renderWithProviders(<AppRoutes />, "/login");
    await user.click(await screen.findByRole("button", { name: "Log in with Telegram code" }));
    const open = await screen.findByRole("link", { name: "Open the Kyber bot in Telegram" });
    expect(open).toHaveAttribute("href", "https://t.me/kyber_bot?start=abc");
    expect(open).toHaveAttribute("target", "_blank");
    const input = screen.getByLabelText("Code from Telegram");
    expect(input).toHaveAttribute("autocomplete", "one-time-code");
    await user.type(input, "000000");
    await user.click(screen.getByRole("button", { name: "Verify code" }));
    expect(await screen.findByText("wrong login code")).toBeInTheDocument();
    await user.clear(input);
    await user.type(input, "123456");
    await user.click(screen.getByRole("button", { name: "Verify code" }));
    expect(await screen.findByText(/No projects yet/)).toBeInTheDocument();
    expect(calls.filter((c) => c.key === "POST /api/v1/auth/telegram/otp/verify").at(-1)?.body).toEqual({ id: "c-1", code: "123456" });
  });
});
