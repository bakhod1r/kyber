import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { IssueWatchers } from "./IssueWatchers";

describe("IssueWatchers", () => {
  it("watches and stops watching", async () => {
    let watching = false;
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1/watchers": () => ({
        status: 200,
        body: { watching, items: watching ? [{ user_id: "u-1", name: "Ali" }, { user_id: "u-2", name: "Me" }] : [{ user_id: "u-1", name: "Ali" }] },
      }),
      "POST /api/v1/issues/KYB-1/watchers": () => {
        watching = true;
        return { status: 204 };
      },
      "DELETE /api/v1/issues/KYB-1/watchers": () => {
        watching = false;
        return { status: 204 };
      },
    });
    renderWithProviders(<IssueWatchers issueKey="KYB-1" />);
    expect(await screen.findByText("1 watcher")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Watch" }));
    expect(await screen.findByText("2 watchers")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Stop watching" }));
    await waitFor(() => expect(calls.some((c) => c.key === "DELETE /api/v1/issues/KYB-1/watchers")).toBe(true));
    expect(await screen.findByText("1 watcher")).toBeInTheDocument();
  });

  it("shows a refusal", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1/watchers": { status: 200, body: { watching: false, items: [] } },
      "POST /api/v1/issues/KYB-1/watchers": { status: 404, body: { type: "about:blank", title: "t", status: 404, code: "ISSUE_NOT_FOUND", detail: "gone" } },
    });
    renderWithProviders(<IssueWatchers issueKey="KYB-1" />);
    await userEvent.click(await screen.findByRole("button", { name: "Watch" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
