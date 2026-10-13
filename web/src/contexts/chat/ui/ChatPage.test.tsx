import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { ChatPage } from "./ChatPage";

/** A stand-in for the browser's EventSource that the test drives. */
class FakeEventSource {
  static last: FakeEventSource | undefined;
  listeners = new Map<string, (e: MessageEvent<string>) => void>();
  constructor(readonly url: string) {
    FakeEventSource.last = this;
  }
  addEventListener(type: string, fn: (e: MessageEvent<string>) => void) {
    this.listeners.set(type, fn);
  }
  close() {}
  emit(type: string, data: unknown) {
    this.listeners.get(type)?.(new MessageEvent(type, { data: JSON.stringify(data) }));
  }
}

const general = { id: "c-1", kind: "channel", name: "general", topic: "All hands", private: false, member_ids: ["u-1", "u-2"], member: true, unread: 2, created_at: "2026-10-13T09:00:00Z" };
const random = { ...general, id: "c-2", name: "random", topic: "", member: false, unread: 0, member_ids: ["u-2"] };
const dm = { ...general, id: "c-3", kind: "dm", name: "", topic: "", private: true, unread: 0 };
const message = (id: string, body: string, extra: object = {}) => ({
  id, conversation_id: "c-1", author_id: "u-2", body, created_at: "2026-10-13T09:00:00Z", edited_at: null, deleted: false,
  thread_id: null, reply_count: 0, reactions: [], issues: [], ...extra,
});

function routes() {
  return mockApi({
    "GET /api/v1/me": { status: 200, body: { id: "u-1", email: "ali@x.uz", name: "Ali" } },
    "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p", key: "KYB", name: "Kyber" }] } },
    "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [
      { user_id: "u-1", email: "ali@x.uz", name: "Ali", role: "admin" }, { user_id: "u-2", email: "bobur@x.uz", name: "Bobur", role: "member" }] } },
    "GET /api/v1/conversations": { status: 200, body: { items: [dm, general, random] } },
    "GET /api/v1/conversations/c-1": { status: 200, body: general },
    "GET /api/v1/conversations/c-1/messages": { status: 200, body: { items: [
      message("m-2", "KYB-1 is ready @ali@x.uz", { issues: [{ key: "KYB-1", title: "Login page", status: "done" }], reply_count: 1,
        reactions: [{ emoji: "🎉", user_ids: ["u-2"] }] }),
      message("m-1", "morning", { author_id: "u-1" }),
      message("m-0", "started a huddle", { kind: "huddle_started" })] } },
    "POST /api/v1/conversations/c-1/read": { status: 204 },
    "POST /api/v1/conversations/c-1/typing": { status: 204 },
    "POST /api/v1/conversations/c-1/messages": { status: 201, body: message("m-3", "ship it", { author_id: "u-1" }) },
    "POST /api/v1/messages/m-2/reactions": { status: 200, body: message("m-2", "KYB-1 is ready @ali@x.uz", { reply_count: 1, reactions: [{ emoji: "🎉", user_ids: ["u-1", "u-2"] }] }) },
    "GET /api/v1/messages/m-2/replies": { status: 200, body: { items: [message("m-9", "after lunch", { thread_id: "m-2" })] } },
    "POST /api/v1/messages/m-2/replies": { status: 201, body: message("m-10", "ok", { thread_id: "m-2", author_id: "u-1" }) },
    "POST /api/v1/channels": { status: 201, body: { ...general, id: "c-4", name: "design", unread: 0 } },
    "POST /api/v1/dms": { status: 200, body: dm },
    "POST /api/v1/messages/m-1/pin": { status: 200, body: message("m-1", "morning", { author_id: "u-1", pinned_by: "u-1", pinned_at: "2026-10-13T10:00:00Z" }) },
    "GET /api/v1/conversations/c-1/pins": { status: 200, body: { items: [message("m-1", "morning", { author_id: "u-1", pinned_by: "u-1" })] } },
    "GET /api/v1/chat/search?q=ready": { status: 200, body: { items: [message("m-2", "KYB-1 is ready")] } },
  });
}

function renderChat(path = "/chat/c-1") {
  return renderWithProviders(
    <Routes>
      <Route path="/chat" element={<ChatPage />} />
      <Route path="/chat/:id" element={<ChatPage />} />
    </Routes>,
    path,
  );
}

beforeEach(() => vi.stubGlobal("EventSource", FakeEventSource));
afterEach(() => vi.unstubAllGlobals());

describe("Chat (S53)", () => {
  it("shows channels and DMs with unread counts, and the open channel's history", async () => {
    const calls = routes();
    renderChat();
    const nav = await screen.findByRole("navigation", { name: "Conversations" });
    expect(await within(nav).findByRole("link", { name: /# general.*2 unread/ })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: /# random/ })).toBeInTheDocument();
    expect(await within(nav).findByRole("link", { name: "Bobur" })).toBeInTheDocument();

    const history = await screen.findByRole("log", { name: "Messages" });
    expect(await within(history).findByText("morning")).toBeInTheDocument();
    expect(within(history).getByRole("link", { name: "KYB-1 Login page" })).toHaveAttribute("href", "/projects/KYB?issue=KYB-1");
    expect(within(history).getByText("@ali@x.uz")).toHaveClass("chat-mention");
    expect(within(history).getByRole("button", { name: "1 reply" })).toBeInTheDocument();
    expect(await screen.findByText("All hands")).toBeInTheDocument();
    expect(within(history).getByRole("article", { name: /Huddle at/ })).toHaveTextContent("Bobur started a huddle");
    expect(calls.some((c) => c.key === "POST /api/v1/conversations/c-1/read")).toBe(true);
  });

  it("posts a message, reacts and replies in a thread", async () => {
    const calls = routes();
    const user = userEvent.setup();
    renderChat();
    await screen.findByText("morning");
    await user.type(screen.getByRole("textbox", { name: "Message # general" }), "ship it{Enter}");
    expect(calls.find((c) => c.key === "POST /api/v1/conversations/c-1/messages")?.body).toEqual({ body: "ship it" });
    expect(await screen.findByText("ship it")).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/v1/conversations/c-1/typing")).toBe(true);

    await user.click(screen.getByRole("button", { name: "🎉 1" }));
    expect(await screen.findByRole("button", { name: "🎉 2" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "1 reply" }));
    const thread = await screen.findByRole("complementary", { name: "Thread" });
    expect(await within(thread).findByText("after lunch")).toBeInTheDocument();
    await user.type(within(thread).getByRole("textbox", { name: "Reply" }), "ok{Enter}");
    expect(calls.find((c) => c.key === "POST /api/v1/messages/m-2/replies")?.body).toEqual({ body: "ok" });
  });

  it("receives messages and typing live from the stream", async () => {
    routes();
    renderChat();
    await screen.findByText("morning");
    act(() => FakeEventSource.last?.emit("typing", { conversation_id: "c-1", user_id: "u-2" }));
    expect(await screen.findByText("Bobur is typing…")).toBeInTheDocument();
    act(() => FakeEventSource.last?.emit("message.posted", { ...message("m-5", "live!"), issues: undefined }));
    expect(await screen.findByText("live!")).toBeInTheDocument();
  });

  it("creates a channel and opens a direct message", async () => {
    const calls = routes();
    const user = userEvent.setup();
    renderChat("/chat");
    await user.click(await screen.findByRole("button", { name: "New channel" }));
    await user.type(screen.getByLabelText("Channel name"), "design");
    await user.click(screen.getByLabelText("Private"));
    await user.click(screen.getByRole("button", { name: "Create" }));
    expect(calls.find((c) => c.key === "POST /api/v1/channels")?.body).toEqual({ name: "design", topic: "", private: true });

    await user.click(screen.getByRole("button", { name: "New message" }));
    await user.click(await screen.findByRole("checkbox", { name: "Bobur" }));
    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(calls.find((c) => c.key === "POST /api/v1/dms")?.body).toEqual({ user_ids: ["u-2"] });
  });
});

describe("Chat pins and search (S54a)", () => {
  it("pins a message, lists pins and searches", async () => {
    const calls = routes();
    const user = userEvent.setup();
    renderChat();
    const history = await screen.findByRole("log", { name: "Messages" });
    const morning = (await within(history).findByText("morning")).closest("article")!;
    await user.click(within(morning as HTMLElement).getByRole("button", { name: "Pin" }));
    expect(calls.some((c) => c.key === "POST /api/v1/messages/m-1/pin")).toBe(true);
    expect(await within(morning as HTMLElement).findByLabelText("pinned")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "📌 Pinned" }));
    expect(await within(await screen.findByRole("list", { name: "Pinned messages" })).findByText(/morning/)).toBeInTheDocument();

    await user.type(screen.getByRole("searchbox", { name: "Search messages" }), "ready{Enter}");
    const results = await screen.findByRole("list", { name: "Search results" });
    expect(await within(results).findByRole("link", { name: /# general · Bobur\s*KYB-1 is ready/ })).toHaveAttribute("href", "/chat/c-1");
  });
});
