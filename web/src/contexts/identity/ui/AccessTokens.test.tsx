import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { PersonalToken } from "../application";
import { AccessTokens } from "./AccessTokens";

describe("AccessTokens", () => {
  it("creates a token, shows the secret once and revokes", async () => {
    let items: PersonalToken[] = [{ id: "t-1", name: "CI", created_at: "2026-10-01T00:00:00Z", expires_at: null, last_used_at: null }];
    const calls = mockApi({
      "GET /api/v1/me/tokens": () => ({ status: 200, body: { items } }),
      "POST /api/v1/me/tokens": () => {
        const t = { id: "t-2", name: "Claude Code", created_at: "2026-10-13T00:00:00Z", expires_at: "2027-01-11T00:00:00Z", last_used_at: null };
        items = [t, ...items];
        return { status: 201, body: { ...t, token: "kpat_secret" } };
      },
      "DELETE /api/v1/me/tokens/t-1": () => {
        items = items.filter((t) => t.id !== "t-1");
        return { status: 204 };
      },
    });
    renderWithProviders(<AccessTokens />);
    expect(await screen.findByText("CI")).toBeInTheDocument();
    expect(screen.getAllByText("Never")).toHaveLength(2);

    await userEvent.type(screen.getByLabelText("Token name"), "Claude Code");
    await userEvent.selectOptions(screen.getByLabelText("Expires"), "90");
    await userEvent.click(screen.getByRole("button", { name: "Create token" }));
    expect(await screen.findByLabelText("New token")).toHaveTextContent("kpat_secret");
    expect(calls.find((c) => c.key === "POST /api/v1/me/tokens")?.body).toEqual({ name: "Claude Code", expires_in_days: 90 });
    expect(await screen.findByRole("button", { name: "Revoke Claude Code" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Copy" }));

    await userEvent.click(screen.getByRole("button", { name: "Revoke CI" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Revoke CI" })).not.toBeInTheDocument());
  });

  it("shows refusals", async () => {
    mockApi({
      "GET /api/v1/me/tokens": { status: 200, body: { items: [] } },
      "POST /api/v1/me/tokens": { status: 422, body: { type: "about:blank", title: "t", status: 422, code: "VALIDATION_FAILED", detail: "name too long" } },
    });
    renderWithProviders(<AccessTokens />);
    expect(screen.getByRole("button", { name: "Create token" })).toBeDisabled();
    await userEvent.type(screen.getByLabelText("Token name"), "x");
    await userEvent.click(screen.getByRole("button", { name: "Create token" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("name too long");
  });
});
