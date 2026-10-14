// notify: domain model (types and pure rules) — no I/O.
export type NotificationKind = "assigned" | "commented" | "mentioned" | "transitioned" | "announcement" | "meeting";
export type Notification = {
  id: string;
  kind: NotificationKind;
  issue_key: string;
  issue_title: string;
  /** The announced project ("" for issue notifications and workspace announcements). */
  project_key?: string;
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
  /** IANA zone for meeting times in email/Telegram; "" = UTC. */
  time_zone?: string;
};

/** Where a notification leads: the issue, a project's announcements, or home for the workspace's. */
export function notificationTarget(n: Pick<Notification, "kind" | "issue_key" | "project_key">): string {
  if (n.kind === "meeting") return "/meetings";
  if (n.kind === "announcement") return n.project_key ? `/projects/${encodeURIComponent(n.project_key)}/announcements` : "/";
  const project = n.issue_key.slice(0, n.issue_key.lastIndexOf("-"));
  return `/projects/${encodeURIComponent(project)}?issue=${encodeURIComponent(n.issue_key)}`;
}
