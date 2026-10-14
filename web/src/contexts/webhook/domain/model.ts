// webhook: domain model — a project's outgoing webhooks, their deliveries, and development links
// (commits and pull requests that mention an issue key).
export type Webhook = {
  id: string;
  project: string;
  url: string;
  events: string[];
  active: boolean;
  created_by: string;
  created_at: string;
  /** Only in the response that creates the webhook: shown once. */
  secret?: string;
};
export type WebhookInput = { url: string; events: string[]; active?: boolean };
export type WebhookDelivery = {
  id: string;
  event: string;
  status: "pending" | "delivered" | "failed";
  attempts: number;
  last_status: number;
  last_error: string;
  created_at: string;
  next_at?: string;
  delivered_at?: string;
};
export type DevelopmentLink = {
  kind: "commit" | "pull_request";
  repo: string;
  ref: string;
  url: string;
  title: string;
  state: string;
  updated_at: string;
};

/** ALL_EVENTS subscribes a webhook to every event of its project. */
export const ALL_EVENTS = "*";

/** Project events a webhook can subscribe to. */
export const WEBHOOK_EVENTS = [
  "issue.created",
  "issue.changed",
  "issue.transitioned",
  "issue.assigned",
  "comment.added",
  "issue.linked",
  "issue.sprint_changed",
  "sprint.started",
  "sprint.completed",
  "version.released",
] as const;

/** parseEvents reads a comma- or space-separated list; empty means every event. */
export function parseEvents(text: string): string[] {
  const list = text.split(/[\s,]+/).filter(Boolean);
  return list.length === 0 ? [ALL_EVENTS] : [...new Set(list)];
}

/** shortRef abbreviates a commit SHA; pull requests read as #number. */
export function shortRef(l: Pick<DevelopmentLink, "kind" | "ref">): string {
  return l.kind === "commit" ? l.ref.slice(0, 7) : `#${l.ref}`;
}
