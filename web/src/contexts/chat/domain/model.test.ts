import { describe, expect, it } from "vitest";
import { applyEvent, conversationTitle, type Message, splitMentions } from "./model";

const msg = (id: string, body: string, extra: Partial<Message> = {}): Message => ({
  id, conversation_id: "c-1", author_id: "u-1", body, created_at: "2026-10-13T09:00:00Z", edited_at: null, deleted: false,
  thread_id: null, reply_count: 0, reactions: [], issues: [], ...extra,
});

describe("chat domain", () => {
  it("applies stream events to a newest-first history", () => {
    let list = [msg("m-1", "hi")];
    list = applyEvent(list, "c-1", { type: "message.posted", message: msg("m-2", "there") });
    expect(list.map((m) => m.id)).toEqual(["m-2", "m-1"]);
    list = applyEvent(list, "c-1", { type: "message.posted", message: msg("m-2", "there") });
    expect(list).toHaveLength(2); // the poster's own echo is not duplicated
    list = applyEvent(list, "c-1", { type: "message.edited", message: msg("m-1", "hello", { edited_at: "x" }) });
    expect(list[1]?.body).toBe("hello");
    list = applyEvent(list, "c-1", { type: "message.posted", message: msg("m-3", "elsewhere", { conversation_id: "c-2" }) });
    expect(list).toHaveLength(2);
    // A reply bumps its parent's reply count but stays out of the channel history.
    list = applyEvent(list, "c-1", { type: "message.posted", message: msg("m-4", "re", { thread_id: "m-1" }) });
    expect(list.map((m) => m.id)).toEqual(["m-2", "m-1"]);
    expect(list[1]?.reply_count).toBe(1);
  });

  it("keeps previews when an event (which carries none) updates a message", () => {
    const withIssue = msg("m-1", "KYB-1", { issues: [{ key: "KYB-1", title: "Login", status: "todo" }] });
    const { issues: _none, ...streamed } = msg("m-1", "KYB-1", { reactions: [{ emoji: "🎉", user_ids: ["u-2"] }] });
    const list = applyEvent([withIssue], "c-1", { type: "message.reacted", message: streamed });
    expect(list[0]?.issues).toHaveLength(1);
    expect(list[0]?.reactions).toHaveLength(1);
  });

  it("names channels and DMs", () => {
    const names = new Map([["u-1", "Ali"], ["u-2", "Bobur"], ["u-3", "Vali"]]);
    expect(conversationTitle({ kind: "channel", name: "general", member_ids: [] }, names, "u-1")).toBe("# general");
    expect(conversationTitle({ kind: "dm", name: "", member_ids: ["u-1", "u-2", "u-3"] }, names, "u-1")).toBe("Bobur, Vali");
    expect(conversationTitle({ kind: "dm", name: "", member_ids: ["u-1", "u-9"] }, names, "u-1")).toBe("Someone");
  });

  it("splits a body into text, mentions and issue keys", () => {
    expect(splitMentions("ping @a@x.uz about KYB-12 now")).toEqual([
      { kind: "text", text: "ping " }, { kind: "mention", text: "@a@x.uz" }, { kind: "text", text: " about " },
      { kind: "issue", text: "KYB-12" }, { kind: "text", text: " now" },
    ]);
  });
});
