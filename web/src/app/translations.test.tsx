// Text the user reads comes from the dictionaries: these screens rendered in Russian show no English.
import { screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";
import { PermissionScheme } from "../contexts/access/ui/PermissionScheme";
import { Board } from "../contexts/agile/ui/Board";
import { AccessTokens } from "../contexts/identity/ui/AccessTokens";
import { QuickFilters } from "../contexts/search/ui/QuickFilters";
import { ChatMentions } from "../contexts/notify/ui/ChatMentions";
import { I18nProvider } from "../shared/i18n/i18n";
import { ru } from "../shared/i18n/ru";
import { mockApi, renderWithProviders } from "../shared/test/test-utils";

const inRussian = (ui: ReactElement) => renderWithProviders(<I18nProvider initial="ru">{ui}</I18nProvider>);

describe("translated screens (ru)", () => {
  it("board: default column names and quick filters", async () => {
    mockApi({ "GET /api/v1/projects/KYB/issues": { status: 200, body: { items: [] } } });
    inRussian(<Board projectKey="KYB" />);
    expect(await screen.findByRole("region", { name: ru["To Do"] })).toBeInTheDocument();
  });

  it("quick filters", async () => {
    mockApi({ "GET /api/v1/filters": { status: 200, body: { items: [] } } });
    inRussian(<QuickFilters projectKey="KYB" value={{ quick: [], saved: "" }} onChange={() => {}} onKeys={() => {}} />);
    expect(await screen.findByRole("button", { name: ru["Only my issues"] })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: ru["Not done"] })).toBeInTheDocument();
  });

  it("permission scheme: permission names", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/permissions": { status: 200, body: { custom: false, permissions: [{ permission: "CREATE_ISSUES", grants: [] }] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
    });
    inRussian(<PermissionScheme projectKey="KYB" />);
    expect(await screen.findByRole("rowheader", { name: ru["Create issues"] })).toBeInTheDocument();
  });

  it("chat mentions: the empty state", async () => {
    mockApi({ "GET /api/v1/notifications": { status: 200, body: { items: [], unread: 0 } } });
    inRussian(<ChatMentions />);
    expect(await screen.findByText(ru["No mentions yet."])).toBeInTheDocument();
  });

  it("access tokens: the explanation", async () => {
    mockApi({ "GET /api/v1/me/tokens": { status: 200, body: { items: [] } } });
    inRussian(<AccessTokens />);
    expect(await screen.findByText(/скрипт/)).toBeInTheDocument();
  });
});
