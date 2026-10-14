import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Comment } from "../application";
import { CommentReactions } from "./CommentReactions";

const comment: Comment = {
  id: "c-1", author_id: "u-1", author_name: "Ali", body: "hi", created_at: "2026-01-01T10:00:00Z",
  reactions: [
    { emoji: "👍", count: 2, mine: true },
    { emoji: "🚀", count: 1, mine: false },
  ],
};

describe("CommentReactions", () => {
  it("shows counts and toggles the caller's reaction", async () => {
    const calls = mockApi({
      [`DELETE /api/v1/comments/c-1/reactions/${encodeURIComponent("👍")}`]: { status: 204 },
      [`PUT /api/v1/comments/c-1/reactions/${encodeURIComponent("🚀")}`]: { status: 204 },
      [`PUT /api/v1/comments/c-1/reactions/${encodeURIComponent("🎉")}`]: { status: 204 },
    });
    renderWithProviders(<CommentReactions issueKey="KYB-1" comment={comment} />);
    const thumbs = screen.getByRole("button", { name: "👍 reacted by 2" });
    expect(thumbs).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "🚀 reacted by 1" })).toHaveAttribute("aria-pressed", "false");

    await userEvent.click(thumbs);
    await waitFor(() => expect(calls.at(-1)?.key).toBe(`DELETE /api/v1/comments/c-1/reactions/${encodeURIComponent("👍")}`));
    await userEvent.click(screen.getByRole("button", { name: "🚀 reacted by 1" }));
    await waitFor(() => expect(calls.at(-1)?.key).toBe(`PUT /api/v1/comments/c-1/reactions/${encodeURIComponent("🚀")}`));

    await userEvent.click(screen.getByRole("button", { name: "Add reaction" }));
    expect(screen.getAllByRole("button", { name: /^React with / })).toHaveLength(8);
    await userEvent.click(screen.getByRole("button", { name: "React with 🎉" }));
    await waitFor(() => expect(calls.at(-1)?.key).toBe(`PUT /api/v1/comments/c-1/reactions/${encodeURIComponent("🎉")}`));
    expect(screen.queryByRole("button", { name: "React with 🎉" })).not.toBeInTheDocument();
  });

  it("picking an emoji the caller already used takes it back, and shows refusals", async () => {
    mockApi({
      [`DELETE /api/v1/comments/c-1/reactions/${encodeURIComponent("👍")}`]: {
        status: 404, body: { type: "about:blank", title: "t", status: 404, code: "COMMENT_NOT_FOUND", detail: "gone" },
      },
    });
    renderWithProviders(<CommentReactions issueKey="KYB-1" comment={comment} />);
    await userEvent.click(screen.getByRole("button", { name: "Add reaction" }));
    await userEvent.click(screen.getByRole("button", { name: "React with 👍" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
