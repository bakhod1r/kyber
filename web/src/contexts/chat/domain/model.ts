// chat: domain model (types and pure rules) — no I/O. Slack-style channels and DMs (ADR-0009).
export type Conversation = {
  id: string; kind: "channel" | "dm"; name: string; topic: string; private: boolean;
  member_ids: string[]; member: boolean; unread: number; created_at: string;
};
export type Reaction = { emoji: string; user_ids: string[] };
export type IssuePreview = { key: string; title: string; status: string };
export type Message = {
  id: string; conversation_id: string; author_id: string; body: string; created_at: string; edited_at: string | null;
  deleted: boolean; thread_id: string | null; reply_count: number; reactions: Reaction[]; issues: IssuePreview[];
  /** System messages (a huddle started or ended); people's messages have none. */
  kind?: "" | "huddle_started" | "huddle_ended";
  pinned_by?: string | null;
  pinned_at?: string | null;
};

/** Events on the realtime stream (GET /api/v1/stream). Stream messages carry no issue previews. */
export type ChatEvent =
  | { type: "message.posted" | "message.edited" | "message.deleted" | "message.reacted" | "message.pinned"; message: Omit<Message, "issues"> & { issues?: IssuePreview[] } }
  | { type: "typing"; conversation_id: string; user_id: string };

/** Applies a stream event to a conversation's newest-first, top-level history. */
export function applyEvent(list: Message[], conversationId: string, e: ChatEvent): Message[] {
  if (e.type === "typing" || e.message.conversation_id !== conversationId) return list;
  const m = e.message;
  if (m.thread_id) {
    if (e.type !== "message.posted") return list;
    return list.map((p) => (p.id === m.thread_id ? { ...p, reply_count: p.reply_count + 1 } : p));
  }
  const i = list.findIndex((x) => x.id === m.id);
  if (i < 0) return e.type === "message.posted" ? [{ ...m, issues: m.issues ?? [] }, ...list] : list;
  return list.map((x, j) => (j === i ? { ...x, ...m, issues: m.issues ?? x.issues } : x));
}

/** "# general" for channels; the other members' names for DMs. */
export function conversationTitle(c: Pick<Conversation, "kind" | "name" | "member_ids">, names: Map<string, string>, me: string): string {
  if (c.kind === "channel") return `# ${c.name}`;
  const others = c.member_ids.filter((id) => id !== me).map((id) => names.get(id) ?? "Someone");
  return [...new Set(others)].join(", ") || "Just you";
}

export type Segment = { kind: "text" | "mention" | "issue"; text: string };

/** Splits a message body into plain text, @email mentions and issue keys (rendered highlighted). */
export function splitMentions(body: string): Segment[] {
  const out: Segment[] = [];
  const re = /(@[^\s@]+@[^\s@]+\.[A-Za-z]{2,})|\b([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)\b/g;
  let last = 0;
  for (const m of body.matchAll(re)) {
    const at = m.index ?? 0;
    if (at > last) out.push({ kind: "text", text: body.slice(last, at) });
    out.push({ kind: m[1] ? "mention" : "issue", text: m[0] });
    last = at + m[0].length;
  }
  if (last < body.length) out.push({ kind: "text", text: body.slice(last) });
  return out;
}
