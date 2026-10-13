import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Announcement } from "../application";
import { Announcements } from "./Announcements";

const release: Announcement = {
  id: "a-1", project_key: "KYB", title: "Release Friday", body: "Deploy at **16:00**", author_id: "u-1", created_at: "2026-10-13T09:00:00Z",
};
const forbidden = { type: "about:blank", title: "t", status: 403, code: "FORBIDDEN", detail: "only admins may publish announcements here" };

describe("announcements", () => {
  it("publishes to a project and shows the feed", async () => {
    let items: Announcement[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/announcements": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/announcements": () => {
        items = [release];
        return { status: 201, body: release };
      },
    });
    renderWithProviders(<Announcements projectKey="KYB" />);
    expect(await screen.findByText("No announcements yet.")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New announcement" }));
    await userEvent.type(screen.getByLabelText("Headline"), "Release Friday");
    await userEvent.type(screen.getByLabelText("Message (Markdown)"), "Deploy at **16:00**");
    await userEvent.click(screen.getByRole("button", { name: "Publish to everyone" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/announcements")?.body).toEqual({ title: "Release Friday", body: "Deploy at **16:00**" }),
    );
    const article = await screen.findByRole("article", { name: "Release Friday" });
    expect(within(article).getByText("16:00").tagName).toBe("STRONG");
    expect(screen.queryByLabelText("Headline")).toBeNull();
  });

  it("uses the workspace feed and explains who may publish", async () => {
    mockApi({
      "GET /api/v1/announcements": { status: 200, body: { items: [{ ...release, project_key: null, body: "" }] } },
      "POST /api/v1/announcements": { status: 403, body: forbidden },
    });
    renderWithProviders(<Announcements projectKey={null} />);
    expect(await screen.findByRole("region", { name: "Workspace announcements" })).toBeInTheDocument();
    expect(await screen.findByRole("article", { name: "Release Friday" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New announcement" }));
    await userEvent.type(screen.getByLabelText("Headline"), "Holiday");
    await userEvent.click(screen.getByRole("button", { name: "Publish to everyone" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("only admins");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByLabelText("Headline")).toBeNull();
  });

  it("shows a loading failure", async () => {
    mockApi({ "GET /api/v1/projects/KYB/announcements": { status: 404, body: { ...forbidden, status: 404, detail: "gone" } } });
    renderWithProviders(<Announcements projectKey="KYB" />);
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
