import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { OAuthConsent } from "./OAuthConsent";

const query = "?response_type=code&client_id=c-1&redirect_uri=http%3A%2F%2Flocalhost%3A7777%2Fcb&code_challenge=abc&code_challenge_method=S256&state=s";

describe("OAuthConsent (ADR-0016)", () => {
  it("names the client and sends the browser back with the code on approval", async () => {
    const calls = mockApi({
      "GET /api/v1/oauth/clients/c-1": { status: 200, body: { client_id: "c-1", client_name: "Claude", redirect_uris: ["http://localhost:7777/cb"] } },
      "POST /api/v1/oauth/authorize": { status: 200, body: { redirect_to: "http://localhost:7777/cb?code=xyz&state=s" } },
    });
    const go = vi.fn();
    renderWithProviders(<OAuthConsent go={go} />, `/oauth/authorize${query}`);
    expect(await screen.findByRole("heading", { name: "Allow Claude to access Kyber?" })).toBeInTheDocument();
    expect(screen.getByText("localhost:7777")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Allow" }));
    await waitFor(() => expect(go).toHaveBeenCalledWith("http://localhost:7777/cb?code=xyz&state=s"));
    expect(calls.find((c) => c.key === "POST /api/v1/oauth/authorize")?.body).toEqual({
      client_id: "c-1", redirect_uri: "http://localhost:7777/cb", code_challenge: "abc", code_challenge_method: "S256", state: "s",
    });
  });

  it("denies with access_denied and rejects malformed requests", async () => {
    mockApi({ "GET /api/v1/oauth/clients/c-1": { status: 200, body: { client_id: "c-1", client_name: "Claude", redirect_uris: ["http://localhost:7777/cb"] } } });
    const go = vi.fn();
    renderWithProviders(<OAuthConsent go={go} />, `/oauth/authorize${query}`);
    await userEvent.click(await screen.findByRole("button", { name: "Deny" }));
    expect(go).toHaveBeenCalledWith("http://localhost:7777/cb?error=access_denied&state=s");

    renderWithProviders(<OAuthConsent go={go} />, "/oauth/authorize?client_id=c-1");
    expect(await screen.findByText("This authorization request is incomplete.")).toBeInTheDocument();
  });
});
