import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Page } from "../application";
import { Docs } from "./Docs";

const page: Page = {
  id: "p-1", project_key: "KYB", title: "Guide", body: "# Welcome\n\nRead **this** and [docs](https://x.uz).\n\n[bad](javascript:alert(1))",
  author_id: "u-1", updated_by: "u-1", updated_at: "2026-10-13T09:00:00Z", version: 1,
};
const conflict = { type: "about:blank", title: "t", status: 409, code: "PAGE_CONFLICT", detail: "changed" };

describe("docs", () => {
  it("writes a page, renders Markdown safely, edits it and shows history", async () => {
    let items: Page[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/docs": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/docs": () => {
        items = [page];
        return { status: 201, body: page };
      },
      "PUT /api/v1/docs/p-1": () => {
        items = [{ ...page, body: "## Second", version: 2 }];
        return { status: 200, body: items[0] };
      },
      "GET /api/v1/docs/p-1/history": {
        status: 200,
        body: { items: [{ number: 2, title: "Guide", body: "## Second", by: "u-1", at: "2026-10-13T10:00:00Z" }, { number: 1, title: "Guide", body: "old *text*", by: "u-1", at: "2026-10-13T09:00:00Z" }] },
      },
      "DELETE /api/v1/docs/p-1": () => {
        items = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<Docs projectKey="KYB" />);
    expect(await screen.findByText("No pages yet.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New page" }));
    await userEvent.type(screen.getByLabelText("Page title"), "Guide");
    await userEvent.type(screen.getByLabelText("Content (Markdown)"), "# Welcome");
    await userEvent.click(screen.getByRole("button", { name: "Preview" }));
    expect(screen.getByRole("heading", { name: "Welcome" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Write" }));
    await userEvent.click(screen.getByRole("button", { name: "Save page" }));
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/docs")?.body).toEqual({ title: "Guide", body: "# Welcome" });

    const article = await screen.findByRole("article", { name: "Guide" });
    expect(within(article).getByRole("link", { name: "docs" })).toHaveAttribute("href", "https://x.uz");
    expect(within(article).queryByRole("link", { name: "bad" })).toBeNull();
    expect(within(article).getByText("this").tagName).toBe("STRONG");

    await userEvent.click(within(article).getByRole("button", { name: "Edit" }));
    const body = screen.getByLabelText("Content (Markdown)");
    await userEvent.clear(body);
    await userEvent.type(body, "## Second");
    await userEvent.click(screen.getByRole("button", { name: "Save page" }));
    await waitFor(() => expect(calls.find((c) => c.key === "PUT /api/v1/docs/p-1")?.body).toEqual({ version: 1, title: "Guide", body: "## Second" }));
    expect(await screen.findByText(/Version 2/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "History" }));
    await userEvent.click(await screen.findByRole("button", { name: "Version 1" }));
    expect(screen.getByText("text").tagName).toBe("EM");
    await userEvent.click(screen.getByRole("button", { name: "Hide history" }));

    await userEvent.click(screen.getByRole("button", { name: "Delete page" }));
    expect(await screen.findByText("No pages yet.")).toBeInTheDocument();
  });

  it("selects pages, cancels edits and reports failures", async () => {
    const other = { ...page, id: "p-2", title: "Runbook", body: "- one\n1. two\n> quote\n\n```\ncode\n```\n---\n`x`" };
    mockApi({
      "GET /api/v1/projects/KYB/docs": { status: 200, body: { items: [page, other] } },
      "PUT /api/v1/docs/p-2": { status: 409, body: conflict },
      "DELETE /api/v1/docs/p-2": { status: 409, body: conflict },
    });
    renderWithProviders(<Docs projectKey="KYB" />);
    await userEvent.click(await screen.findByRole("button", { name: "Runbook" }));
    const article = screen.getByRole("article", { name: "Runbook" });
    expect(within(article).getAllByRole("list")).toHaveLength(2);
    expect(within(article).getByText("quote").tagName).toBe("BLOCKQUOTE");
    expect(within(article).getByText("code").closest("pre")).not.toBeNull();
    await userEvent.click(within(article).getByRole("button", { name: "Edit" }));
    await userEvent.click(screen.getByRole("button", { name: "Save page" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("changed");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete page" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("changed");
  });

  it("comments on a paragraph (S86)", async () => {
    const two = { ...page, body: "# Welcome\n\nCache it here" };
    let comments = [{ id: "c-1", page_id: "p-1", paragraph: 1, quote: "Cache it here", author_id: "u-2", body: "Why?", created_at: "2026-10-13T09:00:00Z" },
      { id: "c-0", page_id: "p-1", paragraph: 5, quote: "Gone", author_id: "u-2", body: "Old remark", created_at: "2026-10-13T08:00:00Z" }];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/docs": { status: 200, body: { items: [two] } },
      "GET /api/v1/docs/p-1/comments": () => ({ status: 200, body: { items: comments } }),
      "POST /api/v1/docs/p-1/comments": () => {
        comments = [...comments, { ...comments[0]!, id: "c-2", author_id: "me", body: "Because" }];
        return { status: 201, body: comments[2] };
      },
      "DELETE /api/v1/doc-comments/c-2": () => {
        comments = comments.filter((c) => c.id !== "c-2");
        return { status: 204 };
      },
    });
    renderWithProviders(<Docs projectKey="KYB" me="me" names={new Map([["u-2", "Vic"]])} />);
    const pin = await screen.findByRole("button", { name: "1 comment on paragraph 2" });
    expect(screen.getByRole("button", { name: "Comment on paragraph 1" })).toBeInTheDocument();
    expect(screen.getByText(/Old remark/)).toBeInTheDocument(); // on an earlier version
    await userEvent.click(pin);
    const panel = screen.getByRole("complementary", { name: "Comments on paragraph 2" });
    expect(within(panel).getByText("Vic")).toBeInTheDocument();
    await userEvent.type(within(panel).getByLabelText("Comment"), "Because");
    await userEvent.click(within(panel).getByRole("button", { name: "Comment" }));
    await waitFor(() => expect(calls.find((c) => c.key === "POST /api/v1/docs/p-1/comments")?.body).toEqual({ paragraph: 1, quote: "Cache it here", body: "Because" }));
    await userEvent.click(await within(panel).findByRole("button", { name: "Remove" }));
    await waitFor(() => expect(calls.some((c) => c.key === "DELETE /api/v1/doc-comments/c-2")).toBe(true));
  });

  it("shows a loading failure", async () => {
    mockApi({ "GET /api/v1/projects/KYB/docs": { status: 404, body: { ...conflict, status: 404, code: "PROJECT_NOT_FOUND", detail: "gone" } } });
    renderWithProviders(<Docs projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
