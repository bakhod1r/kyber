// chat: HTTP gateway to the chat API and the realtime stream (infrastructure).
import { ApiError, isProblem, request, seg, type List } from "../../../shared/http";
import { onConnect, subscribe } from "../../../shared/stream";

export { onConnect };
import type { ChatEvent, Conversation, Message, SavedItem } from "../domain/model";

const conv = (id: string) => `/api/v1/conversations/${seg(id)}`;
const msg = (id: string) => `/api/v1/messages/${seg(id)}`;

export const chatGateway = {
  conversations: () => request<List<Conversation>>("GET", "/api/v1/conversations").then((r) => r.items),
  conversation: (id: string) => request<Conversation>("GET", conv(id)),
  createChannel: (c: { name: string; topic?: string; private?: boolean }) => request<Conversation>("POST", "/api/v1/channels", c),
  openDM: (userIds: string[]) => request<Conversation>("POST", "/api/v1/dms", { user_ids: userIds }),
  join: (id: string) => request<undefined>("POST", `${conv(id)}/join`),
  leave: (id: string) => request<undefined>("POST", `${conv(id)}/leave`),
  invite: (id: string, userId: string) => request<undefined>("POST", `${conv(id)}/members`, { user_id: userId }),
  markRead: (id: string) => request<undefined>("POST", `${conv(id)}/read`),
  typing: (id: string) => request<undefined>("POST", `${conv(id)}/typing`),
  history: (id: string, before?: string) =>
    request<List<Message>>("GET", `${conv(id)}/messages${before ? `?before=${encodeURIComponent(before)}` : ""}`).then((r) => r.items),
  post: (id: string, body: string) => request<Message>("POST", `${conv(id)}/messages`, { body }),
  edit: (id: string, body: string) => request<Message>("PATCH", msg(id), { body }),
  remove: (id: string) => request<undefined>("DELETE", msg(id)),
  replies: (id: string) => request<List<Message>>("GET", `${msg(id)}/replies`).then((r) => r.items),
  reply: (id: string, body: string) => request<Message>("POST", `${msg(id)}/replies`, { body }),
  react: (id: string, emoji: string) => request<Message>("POST", `${msg(id)}/reactions`, { emoji }),
  pin: (id: string) => request<Message>("POST", `${msg(id)}/pin`),
  unpin: (id: string) => request<Message>("DELETE", `${msg(id)}/pin`),
  pins: (id: string) => request<List<Message>>("GET", `${conv(id)}/pins`).then((r) => r.items),
  save: (id: string, remindAt: Date | null) => request<{ saved_at: string; remind_at: string | null }>("PUT", `${msg(id)}/save`, { remind_at: remindAt?.toISOString() ?? null }),
  unsave: (id: string) => request<undefined>("DELETE", `${msg(id)}/save`),
  saved: () => request<List<SavedItem>>("GET", "/api/v1/chat/saved").then((r) => r.items),
  search: (q: string) => request<List<Message>>("GET", `/api/v1/chat/search?q=${encodeURIComponent(q)}`).then((r) => r.items),
  /** Uploads a file as a message (raw body; the header satisfies the CSRF guard for cookie sessions). */
  upload: async (id: string, file: Blob, name: string): Promise<Message> => {
    const res = await fetch(`${conv(id)}/files?name=${encodeURIComponent(name)}`, {
      method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream", "X-Kyber-Request": "1" }, body: file,
    });
    const data: unknown = await res.json().catch(() => undefined);
    if (!res.ok) throw new ApiError(res.status, isProblem(data) ? data.detail || data.title : `upload failed (${res.status})`, isProblem(data) ? data.code : "");
    return data as Message;
  },
  presence: (userIds: string[]) =>
    request<{ online: string[] }>("GET", `/api/v1/presence?${userIds.map((u) => `user_id=${encodeURIComponent(u)}`).join("&")}`).then((r) => r.online),
};

const EVENTS = ["message.posted", "message.edited", "message.deleted", "message.reacted", "message.pinned", "typing"] as const;

/** Chat events from the shared realtime stream. */
export function openChatStream(onEvent: (e: ChatEvent) => void): () => void {
  return subscribe([...EVENTS], (type, data) => {
    if (type === "typing") onEvent({ type, ...(data as { conversation_id: string; user_id: string }) });
    else onEvent({ type: type as Exclude<ChatEvent["type"], "typing">, message: data as Message });
  });
}
