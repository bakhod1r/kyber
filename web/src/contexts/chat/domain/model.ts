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
  kind?: "" | "huddle_started" | "huddle_ended" | "huddle_recording";
  file?: { name: string; type: string; size: number; url: string } | null;
  pinned_by?: string | null;
  pinned_at?: string | null;
};

/** Someone's status (S66): emoji and text, and whether they are in Do Not Disturb. */
export type UserStatus = { user_id: string; emoji: string; text: string; expires_at: string | null; dnd: boolean; dnd_until: string | null };

export const STATUS_PRESETS = [
  { emoji: "📅", text: "In a meeting" }, { emoji: "🎧", text: "Focusing" }, { emoji: "🍽", text: "Lunch" },
  { emoji: "🏠", text: "Working remotely" }, { emoji: "🤒", text: "Out sick" }, { emoji: "🏖", text: "On vacation" },
];

/** "Clear after" choices for a status, from now; null means never. */
export function clearAfter(now: Date): { label: string; at: Date | null }[] {
  const endOfDay = new Date(now);
  endOfDay.setHours(23, 59, 0, 0);
  const plus = (min: number) => new Date(now.getTime() + min * 60_000);
  return [
    { label: "30 minutes", at: plus(30) }, { label: "1 hour", at: plus(60) }, { label: "4 hours", at: plus(240) },
    { label: "Today", at: endOfDay }, { label: "Don't clear", at: null },
  ];
}

/** A message I saved for later (S65). */
export type SavedItem = { message: Message; saved_at: string; remind_at: string | null; reminded: boolean };

/** When "Save for later" can remind me, from now: in 20 minutes, 1 hour, 3 hours, or tomorrow at 9:00. */
export function remindPresets(now: Date): { label: string; at: Date }[] {
  const tomorrow = new Date(now);
  tomorrow.setDate(now.getDate() + 1);
  tomorrow.setHours(9, 0, 0, 0);
  const plus = (min: number) => new Date(now.getTime() + min * 60_000);
  return [
    { label: "In 20 minutes", at: plus(20) },
    { label: "In 1 hour", at: plus(60) },
    { label: "In 3 hours", at: plus(180) },
    { label: "Tomorrow at 9:00", at: tomorrow },
  ];
}

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

/** 1.2 MB, 340 KB… */
export function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

export const MAX_FILE = 25 * 1024 * 1024;
