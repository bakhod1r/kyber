import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { TelegramGroups } from "./TelegramGroups";

describe("TelegramGroups", () => {
  it("connects a group through the bot and lists it", async () => {
    let items: { chat_id: string; title: string; linked_at: string }[] = [];
    const open = vi.spyOn(window, "open").mockImplementation(() => null);
    mockApi({
      "GET /api/v1/projects/KYB/telegram-groups": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/telegram-groups/link": () => {
        items = [{ chat_id: "-100500", title: "Kyber devs", linked_at: "2026-10-12T09:00:00Z" }]; // the admin picked a group
        return { status: 200, body: { url: "https://t.me/KyberBot?startgroup=g_abc", expires_at: "2026-10-12T09:10:00Z" } };
      },
      "DELETE /api/v1/projects/KYB/telegram-groups/-100500": () => {
        items = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<TelegramGroups projectKey="KYB" />);
    expect(await screen.findByText("No Telegram groups yet.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Connect a Telegram group" }));
    expect(open).toHaveBeenCalledWith("https://t.me/KyberBot?startgroup=g_abc", "_blank", "noopener");
    const list = await screen.findByRole("list", { name: "Connected groups" }, { timeout: 4000 });
    expect(within(list).getByText("Kyber devs")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: "Disconnect Kyber devs" }));
    await waitFor(() => expect(screen.queryByRole("list", { name: "Connected groups" })).not.toBeInTheDocument());
    open.mockRestore();
  });

  it("explains when only admins may manage groups or no bot is configured", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/telegram-groups": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "x" } },
    });
    renderWithProviders(<TelegramGroups projectKey="KYB" />);
    expect(await screen.findByText(/Only project admins/)).toBeInTheDocument();
  });
});
