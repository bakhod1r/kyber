import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Layout } from "./Layout";
import { mockApi, renderWithProviders } from "../shared/test/test-utils";
import { I18nProvider } from "../shared/i18n/i18n";
import { ru } from "../shared/i18n/ru";

const me = { id: "u-1", email: "ali@x.uz", name: "Ali" };
const noAdmin = { "GET /api/v1/admin/access": { status: 403, body: { type: "about:blank", title: "t", status: 403, code: "FORBIDDEN", detail: "no" } } };

describe("Layout top bar", () => {
  it("marks the section you are in", async () => {
    mockApi(noAdmin);
    renderWithProviders(<Layout user={me}>x</Layout>, "/search");
    const nav = screen.getByRole("navigation", { name: "Main" });
    expect(within(nav).getByRole("link", { name: "Search" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "Chat" })).not.toHaveAttribute("aria-current");
  });

  it("translates every navigation label", async () => {
    mockApi(noAdmin);
    renderWithProviders(<I18nProvider initial="ru"><Layout user={me}>x</Layout></I18nProvider>, "/");
    const nav = screen.getByRole("navigation", { name: ru["Main"] });
    expect(within(nav).getByRole("link", { name: ru["Chat"] })).toBeInTheDocument();
  });

  it("the user's name opens their settings and says so", async () => {
    mockApi(noAdmin);
    renderWithProviders(<Layout user={me}>x</Layout>, "/");
    expect(screen.getByRole("link", { name: "Ali" })).toHaveAttribute("title", "Settings");
  });
});
