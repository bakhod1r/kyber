// notify: HTTP gateway to the notify API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Inbox, NotificationSettings } from "../domain/model";

export const notifyGateway = {
  notifications: () => request<Inbox>("GET", "/api/v1/notifications"),
  markNotificationRead: (id: string) => request<undefined>("POST", `/api/v1/notifications/${seg(id)}/read`),
  markAllNotificationsRead: () => request<undefined>("POST", "/api/v1/notifications/read-all"),
  /** Reads my chat mentions in one conversation (I opened it). */
  markConversationRead: (conversationId: string) =>
    request<undefined>("POST", `/api/v1/notifications/read-all?conversation_id=${encodeURIComponent(conversationId)}`),
  notificationSettings: () => request<NotificationSettings>("GET", "/api/v1/me/notification-settings"),
  setNotificationSettings: (patch: Partial<Pick<NotificationSettings, "email" | "telegram">>) =>
    request<NotificationSettings>("PUT", "/api/v1/me/notification-settings", patch),
  telegramGroups: (key: string) =>
    request<List<{ chat_id: string; title: string; linked_at: string }>>("GET", `/api/v1/projects/${seg(key)}/telegram-groups`).then((r) => r.items),
  telegramGroupLink: (key: string) =>
    request<{ url: string; expires_at: string }>("POST", `/api/v1/projects/${seg(key)}/telegram-groups/link`),
  unlinkTelegramGroup: (key: string, chat: string) =>
    request<undefined>("DELETE", `/api/v1/projects/${seg(key)}/telegram-groups/${seg(chat)}`),
};
