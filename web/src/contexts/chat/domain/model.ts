// chat: domain model (types and pure rules) — no I/O. Slack-style channels and DMs (ADR-0009).
export type Conversation = {
  id: string; kind: "channel" | "dm"; name: string; topic: string; private: boolean;
  member_ids: string[]; member: boolean; unread: number; created_at: string;
  /** S69: the owner (creator) and admins manage a channel; an archived one is read-only. */
  owner_id?: string; admin_ids?: string[]; archived?: boolean;
  /** S73: the project whose news are posted here ("" when none). */
  project_key?: string;
};

/** Whether u may rename, archive or remove people from the channel (S69). */
export const canManage = (c: Pick<Conversation, "kind" | "owner_id" | "admin_ids">, u: string) =>
  c.kind === "channel" && (c.owner_id === u || (c.admin_ids ?? []).includes(u));
export type Reaction = { emoji: string; user_ids: string[] };
export type IssuePreview = { key: string; title: string; status: string };
export type Message = {
  id: string; conversation_id: string; author_id: string; body: string; created_at: string; edited_at: string | null;
  deleted: boolean; thread_id: string | null; reply_count: number; reactions: Reaction[]; issues: IssuePreview[];
  /** System messages (a huddle started or ended); people's messages have none. */
  kind?: "" | "huddle_started" | "huddle_ended" | "huddle_recording" | "project_event";
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

export type Receipt = { user_id: string; read_at: string };

/** "Seen" (S68): my latest message in a DM was read by someone else (their read cursor reached it). */
export function seen(messages: Pick<Message, "id" | "author_id" | "created_at">[], receipts: Receipt[], me: string): string | null {
  const mine = [...messages].reverse().find((m) => m.author_id === me);
  if (!mine || messages[messages.length - 1]?.id !== mine.id) return null; // only under my latest, last message
  return receipts.some((r) => r.user_id !== me && Date.parse(r.read_at) >= Date.parse(mine.created_at)) ? mine.id : null;
}

/** A slash command for issues (S72), run by the app shell with the user's own permissions. */
export type SlashCommand =
  | { name: "issue"; project: string; title: string }
  | { name: "assign"; issue: string; to: string } // "me" or an e-mail
  | { name: "status"; issue: string; status: string };

export const COMMAND_HELP = "Commands: /issue KEY title · /assign KEY-1 @me|@email · /status KEY-1 Status name · /remind me in 20 minutes …";

/** Reads /issue, /assign and /status; null for anything else (or a malformed one: show COMMAND_HELP). */
export function parseCommand(input: string): SlashCommand | null {
  const t = input.trim();
  let m = /^\/issue\s+([A-Za-z][A-Za-z0-9]{1,9})\s+(.+)$/s.exec(t);
  if (m) return { name: "issue", project: m[1]!.toUpperCase(), title: m[2]!.trim() };
  m = /^\/assign\s+([A-Za-z][A-Za-z0-9]{1,9}-[1-9][0-9]*)\s+@?(\S+)$/.exec(t);
  if (m) return { name: "assign", issue: m[1]!.toUpperCase(), to: m[2]!.toLowerCase() };
  m = /^\/status\s+([A-Za-z][A-Za-z0-9]{1,9}-[1-9][0-9]*)\s+(.+)$/.exec(t);
  if (m) return { name: "status", issue: m[1]!.toUpperCase(), status: m[2]!.trim() };
  return null;
}

/** My settings for a conversation (S71). */
export type Level = "all" | "mentions" | "muted";
export type ConversationPrefs = { conversation_id: string; level: Level; section: string };

/** Sidebar sections (S71): my named sections (sorted), then the default Channels and Direct messages lists. */
export function sections(convs: Conversation[], prefs: ConversationPrefs[]): { title: string; items: Conversation[] }[] {
  const sectionOf = new Map(prefs.map((p) => [p.conversation_id, p.section]));
  const named = [...new Set(prefs.map((p) => p.section).filter(Boolean))].sort((a, b) => a.localeCompare(b));
  const rest = convs.filter((c) => !sectionOf.get(c.id));
  return [
    ...named.map((title) => ({ title, items: convs.filter((c) => sectionOf.get(c.id) === title) })).filter((s) => s.items.length > 0),
    { title: "Channels", items: rest.filter((c) => c.kind === "channel") },
    { title: "Direct messages", items: rest.filter((c) => c.kind === "dm") },
  ];
}

/** A user group (S70): @handle notifies its members. */
export type UserGroup = { id: string; handle: string; name: string; member_ids: string[]; created_by: string; created_at: string };

/** A message I scheduled (S67). */
export type ScheduledMessage = { id: string; conversation_id: string; body: string; send_at: string; created_at: string };

const UNITS: Record<string, number> = { m: 1, min: 1, mins: 1, minute: 1, minutes: 1, h: 60, hour: 60, hours: 60, d: 1440, day: 1440, days: 1440 };

/**
 * Reads "/remind me in 20 minutes to call the bank", "/remind me tomorrow standup notes" (9:00) or
 * "/remind me at 15:30 deploy" (today, or tomorrow once that time has passed). Null when it is not one.
 */
export function parseRemind(input: string, now: Date): { text: string; at: Date } | null {
  const m = /^\/remind\s+me\s+(?:in\s+(\d+)\s*([a-z]+)|(tomorrow)|at\s+(\d{1,2}):(\d{2}))\s+(?:to\s+)?(.+)$/is.exec(input.trim());
  if (!m) return null;
  const text = m[6]!.trim();
  if (m[1]) {
    const unit = UNITS[m[2]!.toLowerCase()];
    if (!unit || Number(m[1]) <= 0) return null;
    return { text, at: new Date(now.getTime() + Number(m[1]) * unit * 60_000) };
  }
  const at = new Date(now);
  if (m[3]) {
    at.setDate(at.getDate() + 1);
    at.setHours(9, 0, 0, 0);
    return { text, at };
  }
  const [h, min] = [Number(m[4]), Number(m[5])];
  if (h > 23 || min > 59) return null;
  at.setHours(h, min, 0, 0);
  if (at <= now) at.setDate(at.getDate() + 1);
  return { text, at };
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
  | { type: "typing"; conversation_id: string; user_id: string }
  | { type: "conversation.read"; conversation_id: string; user_id: string; at: string }
  | { type: "conversation.updated"; conversation_id: string };

/** Applies a stream event to a conversation's newest-first, top-level history. */
export function applyEvent(list: Message[], conversationId: string, e: ChatEvent): Message[] {
  if (e.type === "typing" || e.type === "conversation.read" || e.type === "conversation.updated" || e.message.conversation_id !== conversationId) return list;
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
  // e-mail mentions, issue keys, and @group handles (S70)
  const re = /(@[^\s@]+@[^\s@]+\.[A-Za-z]{2,})|\b([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)\b|(?<![\w@.])(@[a-z0-9][a-z0-9-]{1,29})(?![\w@-])/g;
  let last = 0;
  for (const m of body.matchAll(re)) {
    const at = m.index ?? 0;
    if (at > last) out.push({ kind: "text", text: body.slice(last, at) });
    out.push({ kind: m[2] ? "issue" : "mention", text: m[0] });
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
