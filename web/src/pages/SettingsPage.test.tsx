import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { SettingsPage } from "./SettingsPage";

const settings = (over = {}) => ({
  email: true, telegram: true, email_available: true, telegram_available: true, telegram_connected: false, ...over,
});

describe("Notification settings", () => {
  it("turns channels on and off", async () => {
    const calls = mockApi({
      "GET /api/v1/me/notification-settings": { status: 200, body: settings() },
      "PUT /api/v1/me/notification-settings": (init) => ({ status: 200, body: settings(JSON.parse(String(init?.body))) }),
    });
    renderWithProviders(<SettingsPage />);
    const email = await screen.findByRole("checkbox", { name: /Email/ });
    expect(email).toBeChecked();
    await userEvent.click(email);
    await waitFor(() => expect(calls.find((c) => c.key.startsWith("PUT"))?.body).toEqual({ email: false }));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: /Email/ })).not.toBeChecked());
  });

  it("connects Telegram through the bot link and notices when it is done", async () => {
    let connected = false;
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    mockApi({
      "GET /api/v1/me/notification-settings": () => ({ status: 200, body: settings({ telegram_connected: connected }) }),
      "POST /api/v1/me/telegram/connect": () => {
        connected = true; // the user pressed Start in the bot
        return { status: 200, body: { url: "https://t.me/KyberBot?start=c_abc", expires_at: "2026-10-12T09:10:00Z" } };
      },
    });
    renderWithProviders(<SettingsPage />);
    expect(await screen.findByText("Not connected")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Connect Telegram" }));
    expect(open).toHaveBeenCalledWith("https://t.me/KyberBot?start=c_abc", "_blank", "noopener");
    expect(screen.getByRole("link", { name: /Open the bot/ })).toHaveAttribute("href", "https://t.me/KyberBot?start=c_abc");
    expect(await screen.findByText("Connected", {}, { timeout: 4000 })).toBeInTheDocument();
    open.mockRestore();
  });

  it("explains channels the server has not configured", async () => {
    mockApi({ "GET /api/v1/me/notification-settings": { status: 200, body: settings({ email_available: false, telegram_available: false }) } });
    renderWithProviders(<SettingsPage />);
    expect(await screen.findByText(/Email is not configured on this server/)).toBeInTheDocument();
    expect(screen.getByText(/Telegram is not configured on this server/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Connect Telegram" })).not.toBeInTheDocument();
  });
});
