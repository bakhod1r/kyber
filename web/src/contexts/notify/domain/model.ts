// notify: domain model (types and pure rules) — no I/O.
export type NotificationKind = "assigned" | "commented" | "mentioned" | "transitioned";
export type Notification = {
  id: string;
  kind: NotificationKind;
  issue_key: string;
  issue_title: string;
  actor_name: string;
  excerpt: string;
  read: boolean;
  created_at: string;
};
export type Inbox = { items: Notification[]; unread: number };
export type NotificationSettings = {
  email: boolean;
  telegram: boolean;
  email_available: boolean;
  telegram_available: boolean;
  telegram_connected: boolean;
};
