import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { formatSize, type Attachment } from "../application";
import { IssueAttachments } from "./IssueAttachments";

const file = (over: Partial<Attachment>): Attachment => ({
  id: "a-1",
  issue_key: "KYB-1",
  name: "log.txt",
  content_type: "text/plain",
  size: 2048,
  sha256: "x",
  uploaded_by: "u-1",
  uploaded_at: "2026-10-13T09:00:00Z",
  inline: false,
  url: "/api/v1/attachments/a-1",
  ...over,
});

describe("IssueAttachments", () => {
  it("lists, uploads and removes files", async () => {
    let items = [file({}), file({ id: "a-2", name: "shot.png", content_type: "image/png", inline: true, url: "/api/v1/attachments/a-2" })];
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1/attachments": () => ({ status: 200, body: { items } }),
      "POST /api/v1/issues/KYB-1/attachments": () => {
        items = [...items, file({ id: "a-3", name: "new.txt", size: 10 })];
        return { status: 201, body: items[2] };
      },
      "DELETE /api/v1/attachments/a-1": () => {
        items = items.filter((a) => a.id !== "a-1");
        return { status: 204 };
      },
    });
    renderWithProviders(<IssueAttachments issueKey="KYB-1" />);
    expect(await screen.findByRole("link", { name: "log.txt" })).toHaveAttribute("href", "/api/v1/attachments/a-1?download");
    expect(screen.getByRole("link", { name: "shot.png" })).toHaveAttribute("href", "/api/v1/attachments/a-2");
    expect(screen.getAllByText("2.0 KB")).toHaveLength(2);

    await userEvent.upload(screen.getByLabelText("Attach file"), new File(["hello"], "new.txt", { type: "text/plain" }));
    expect(await screen.findByRole("link", { name: "new.txt" })).toBeInTheDocument();
    const post = calls.find((c) => c.key === "POST /api/v1/issues/KYB-1/attachments");
    expect(post?.headers.get("X-Kyber-Request")).toBe("1");
    expect(post?.body).toBeInstanceOf(FormData);

    await userEvent.click(screen.getByRole("button", { name: "Remove log.txt" }));
    await waitFor(() => expect(screen.queryByRole("link", { name: "log.txt" })).not.toBeInTheDocument());
  });

  it("refuses files over the limit before uploading and shows server refusals", async () => {
    const calls = mockApi({
      "GET /api/v1/issues/KYB-1/attachments": { status: 200, body: { items: [] } },
      "POST /api/v1/issues/KYB-1/attachments": {
        status: 403,
        body: { type: "about:blank", title: "t", status: 403, code: "PROJECT_FORBIDDEN", detail: "insufficient project role" },
      },
    });
    renderWithProviders(<IssueAttachments issueKey="KYB-1" />);
    const input = await screen.findByLabelText("Attach file");
    const big = new File(["x"], "big.bin");
    Object.defineProperty(big, "size", { value: 26 * 1024 * 1024 });
    await userEvent.upload(input, big);
    expect(await screen.findByRole("alert")).toHaveTextContent("big.bin is 26.0 MB; the limit is 25.0 MB.");
    expect(calls.some((c) => c.key.startsWith("POST"))).toBe(false);

    await userEvent.upload(input, new File(["x"], "x.txt"));
    expect(await screen.findByRole("alert")).toHaveTextContent("insufficient project role");
  });

  it("formats sizes", () => {
    expect(formatSize(512)).toBe("512 B");
    expect(formatSize(1536)).toBe("1.5 KB");
    expect(formatSize(3.2 * 1024 * 1024)).toBe("3.2 MB");
  });
});
