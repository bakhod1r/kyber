import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { parseEvents, shortRef, type Webhook } from "../application";
import { Development } from "./Development";
import { Webhooks } from "./Webhooks";

const hook: Webhook = {
  id: "w-1", project: "KYB", url: "https://ci.example.com/kyber", events: ["*"], active: true, created_by: "u-1", created_at: "2026-10-14T09:00:00Z",
};
const base = "/api/v1/projects/KYB/webhooks";

describe("webhooks", () => {
  it("adds a webhook, shows its secret once, toggles, lists deliveries and deletes it", async () => {
    let items: Webhook[] = [];
    const calls = mockApi({
      [`GET ${base}`]: () => ({ status: 200, body: { items } }),
      [`POST ${base}`]: () => {
        items = [hook];
        return { status: 201, body: { ...hook, secret: "abc123" } };
      },
      [`PUT ${base}/w-1`]: () => {
        items = [{ ...hook, active: false }];
        return { status: 200, body: items[0] };
      },
      [`GET ${base}/w-1/deliveries`]: {
        status: 200,
        body: {
          items: [
            { id: "d-2", event: "issue.created", status: "failed", attempts: 8, last_status: 0, last_error: "refused", created_at: "2026-10-14T09:05:00Z" },
            { id: "d-1", event: "issue.created", status: "delivered", attempts: 1, last_status: 200, last_error: "", created_at: "2026-10-14T09:00:00Z" },
            { id: "d-0", event: "issue.changed", status: "pending", attempts: 0, last_status: 0, last_error: "", created_at: "2026-10-14T08:00:00Z" },
          ],
        },
      },
      [`DELETE ${base}/w-1`]: () => {
        items = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<Webhooks projectKey="KYB" />);
    expect(await screen.findByText("No webhooks yet.")).toBeInTheDocument();
    await userEvent.type(screen.getByLabelText("Payload URL"), "https://ci.example.com/kyber");
    await userEvent.click(screen.getByRole("button", { name: "Add webhook" }));
    expect(calls.find((c) => c.key === `POST ${base}`)?.body).toEqual({ url: "https://ci.example.com/kyber", events: ["*"] });
    expect(await screen.findByRole("status")).toHaveTextContent("abc123");

    const list = await screen.findByRole("list", { name: "Webhooks" });
    await userEvent.click(within(list).getByRole("checkbox"));
    await waitFor(() => expect(calls.find((c) => c.key === `PUT ${base}/w-1`)?.body).toEqual({ url: hook.url, events: ["*"], active: false }));

    await userEvent.click(within(list).getByRole("button", { name: "Deliveries" }));
    const table = await screen.findByRole("table", { name: "Recent deliveries" });
    expect(within(table).getByText("Delivered")).toBeInTheDocument();
    expect(within(table).getByText("Failed")).toBeInTheDocument();
    expect(within(table).getByText("Pending")).toBeInTheDocument();
    expect(within(table).getByText("refused")).toBeInTheDocument();
    expect(within(table).getByText("200")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: "Deliveries" }));

    await userEvent.click(screen.getByRole("button", { name: `Delete webhook ${hook.url}` }));
    expect(await screen.findByText("No webhooks yet.")).toBeInTheDocument();
  });

  it("explains that only admins manage webhooks, and shows errors", async () => {
    mockApi({ [`GET ${base}`]: { status: 403, body: { type: "about:blank", title: "Forbidden", status: 403, code: "PROJECT_FORBIDDEN" } } });
    renderWithProviders(<Webhooks projectKey="KYB" />);
    expect(await screen.findByText("Only project admins can manage webhooks.")).toBeInTheDocument();
  });

  it("shows validation problems and an empty delivery list", async () => {
    mockApi({
      [`GET ${base}`]: { status: 200, body: { items: [hook] } },
      [`POST ${base}`]: { status: 422, body: { type: "about:blank", title: "Invalid", status: 422, code: "VALIDATION_FAILED", detail: "plain http is allowed only for localhost" } },
      [`GET ${base}/w-1/deliveries`]: { status: 200, body: { items: [] } },
    });
    renderWithProviders(<Webhooks projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Payload URL"), "http://x.io");
    await userEvent.type(screen.getByLabelText("Events (comma-separated; empty = all)"), "issue.created, comment.added");
    await userEvent.click(screen.getByRole("button", { name: "Add webhook" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("plain http is allowed only for localhost");
    await userEvent.click(screen.getByRole("button", { name: "Deliveries" }));
    expect(await screen.findByText("No deliveries yet.")).toBeInTheDocument();
  });

  it("shows other load errors as they are", async () => {
    mockApi({
      [`GET ${base}`]: { status: 500, body: { type: "about:blank", title: "Boom", status: 500, code: "INTERNAL" } },
    });
    renderWithProviders(<Webhooks projectKey="KYB" />);
    expect(await screen.findByText("Boom")).toBeInTheDocument();
  });
});

describe("development", () => {
  it("lists commits and pull requests that mention the issue", async () => {
    mockApi({
      "GET /api/v1/issues/KYB-1/development": {
        status: 200,
        body: {
          items: [
            { kind: "pull_request", repo: "o/r", ref: "7", url: "https://github.com/o/r/pull/7", title: "Login page", state: "merged", updated_at: "2026-10-14T09:00:00Z" },
            { kind: "commit", repo: "o/r", ref: "abcdef123456", url: "https://github.com/o/r/commit/abcdef1", title: "KYB-1 fix", state: "", updated_at: "2026-10-14T08:00:00Z" },
          ],
        },
      },
    });
    renderWithProviders(<Development issueKey="KYB-1" />);
    const section = await screen.findByRole("region", { name: "Development" });
    expect(within(section).getByRole("link", { name: "o/r #7" })).toHaveAttribute("href", "https://github.com/o/r/pull/7");
    expect(within(section).getByRole("link", { name: "o/r abcdef1" })).toBeInTheDocument();
    expect(within(section).getByText("Merged")).toBeInTheDocument();
  });

  it("renders nothing without links", async () => {
    const calls = mockApi({ "GET /api/v1/issues/KYB-2/development": { status: 200, body: { items: [] } } });
    const { container } = renderWithProviders(<Development issueKey="KYB-2" />);
    await waitFor(() => expect(calls).toHaveLength(1));
    expect(container).toBeEmptyDOMElement();
  });

  it("parses events and abbreviates refs", () => {
    expect(parseEvents(" ")).toEqual(["*"]);
    expect(parseEvents("issue.created, issue.created comment.added")).toEqual(["issue.created", "comment.added"]);
    expect(shortRef({ kind: "commit", ref: "abcdef123" })).toBe("abcdef1");
    expect(shortRef({ kind: "pull_request", ref: "7" })).toBe("#7");
  });
});
