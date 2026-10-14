import { act, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { IssuePresence, PRESENCE_EVERY_MS } from "./IssuePresence";

afterEach(() => vi.useRealTimers());

describe("IssuePresence", () => {
  it("beats while open and lists the other viewers", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let viewers = [{ user_id: "me", name: "Me Myself" }];
    const calls = mockApi({
      "POST /api/v1/issues/KYB-1/presence": { status: 204 },
      "GET /api/v1/issues/KYB-1/presence": () => ({ status: 200, body: { items: viewers } }),
    });
    const { unmount } = renderWithProviders(<IssuePresence issueKey="KYB-1" me="me" />);
    await waitFor(() => expect(calls.filter((c) => c.key === "POST /api/v1/issues/KYB-1/presence")).toHaveLength(1));
    expect(screen.queryByLabelText("Also viewing")).not.toBeInTheDocument();

    viewers = [...viewers, { user_id: "u-2", name: "Ali Valiyev" }];
    await act(() => vi.advanceTimersByTimeAsync(PRESENCE_EVERY_MS));
    expect(await screen.findByLabelText("Ali Valiyev")).toHaveTextContent("AV");
    expect(screen.getByLabelText("Also viewing")).toHaveTextContent("Also viewing:");
    expect(screen.queryByLabelText("Me Myself")).not.toBeInTheDocument();
    expect(calls.filter((c) => c.key === "POST /api/v1/issues/KYB-1/presence")).toHaveLength(2);

    unmount();
    await act(() => vi.advanceTimersByTimeAsync(PRESENCE_EVERY_MS));
    expect(calls.filter((c) => c.key === "POST /api/v1/issues/KYB-1/presence")).toHaveLength(2);
  });

  it("ignores a failed heartbeat", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1/presence": { status: 200, body: { items: [{ user_id: "u-2", name: "Bob" }] } },
    });
    renderWithProviders(<IssuePresence issueKey="KYB-1" />);
    expect(await screen.findByLabelText("Bob")).toHaveTextContent("B");
  });
});
