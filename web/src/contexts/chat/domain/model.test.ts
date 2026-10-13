import { describe, expect, it } from "vitest";
import { applyEvent, conversationTitle, fileSize, type Message, remindPresets, splitMentions, clearAfter, parseRemind, seen } from "./model";

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

describe("files", () => {
  it("formats sizes", () => {
    expect([fileSize(900), fileSize(340 * 1024), fileSize(1.25 * 1024 * 1024)]).toEqual(["900 B", "340 KB", "1.3 MB"]);
  });
});

describe("save for later (S65)", () => {
  it("offers reminders later today and tomorrow morning", () => {
    const now = new Date(2026, 9, 13, 22, 30);
    const p = remindPresets(now);
    expect(p.map((x) => x.label)).toEqual(["In 20 minutes", "In 1 hour", "In 3 hours", "Tomorrow at 9:00"]);
    expect(p[0]!.at.getTime() - now.getTime()).toBe(20 * 60_000);
    expect([p[3]!.at.getDate(), p[3]!.at.getHours(), p[3]!.at.getMinutes()]).toEqual([14, 9, 0]);
  });
});

describe("status (S66)", () => {
  it("clears after a while, at the end of the day, or never", () => {
    const now = new Date(2026, 9, 13, 10, 0);
    const c = clearAfter(now);
    expect(c.map((x) => x.label)).toEqual(["30 minutes", "1 hour", "4 hours", "Today", "Don't clear"]);
    expect(c[3]!.at!.getHours()).toBe(23);
    expect(c[4]!.at).toBeNull();
  });
});

describe("/remind (S67)", () => {
  const now = new Date(2026, 9, 13, 16, 0);
  it("reads in, tomorrow and at", () => {
    expect(parseRemind("/remind me in 20 minutes to call the bank", now)).toEqual({ text: "call the bank", at: new Date(2026, 9, 13, 16, 20) });
    expect(parseRemind("/remind me in 2h deploy", now)?.at).toEqual(new Date(2026, 9, 13, 18, 0));
    expect(parseRemind("/remind me tomorrow standup notes", now)).toEqual({ text: "standup notes", at: new Date(2026, 9, 14, 9, 0) });
    expect(parseRemind("/remind me at 17:30 review", now)?.at).toEqual(new Date(2026, 9, 13, 17, 30));
    expect(parseRemind("/remind me at 9:00 review", now)?.at).toEqual(new Date(2026, 9, 14, 9, 0));
  });
  it("is null for anything else", () => {
    for (const s of ["hello", "/remind me", "/remind me in 5 weeks x", "/remind me at 25:00 x", "/remind you in 5m x"]) expect(parseRemind(s, now)).toBeNull();
  });
});

describe("seen (S68)", () => {
  const msgs = [
    { id: "1", author_id: "dev", created_at: "2026-10-13T09:00:00.000Z" },
    { id: "2", author_id: "me", created_at: "2026-10-13T09:01:00.000Z" },
  ];
  it("marks my last message once someone else read up to it", () => {
    expect(seen(msgs, [{ user_id: "dev", read_at: "2026-10-13T09:00:30.000Z" }], "me")).toBeNull();
    expect(seen(msgs, [{ user_id: "dev", read_at: "2026-10-13T09:02:00.000Z" }], "me")).toBe("2");
    expect(seen(msgs, [{ user_id: "me", read_at: "2026-10-13T09:02:00.000Z" }], "me")).toBeNull();
    expect(seen([...msgs, { id: "3", author_id: "dev", created_at: "2026-10-13T09:03:00.000Z" }], [{ user_id: "dev", read_at: "2026-10-13T09:04:00.000Z" }], "me")).toBeNull();
  });
});

describe("group mentions (S70)", () => {
  it("highlights @handles but not e-mail addresses", () => {
    expect(splitMentions("ping @backend and a@b.uz").map((x) => [x.kind, x.text])).toEqual([
      ["text", "ping "], ["mention", "@backend"], ["text", " and a@b.uz"],
    ]);
  });
});
