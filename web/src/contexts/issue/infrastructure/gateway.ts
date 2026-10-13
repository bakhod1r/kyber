// issue: HTTP gateway to the issue API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Comment, HistoryEntry, Issue, IssueLink, IssuePatch, IssueType, LinkType, Status, Workflow, Worklog } from "../domain/model";

export const issueGateway = {
  /** Issues in backlog rank order; `sprint` filters to a sprint id or "backlog". */
  issues: (key: string, sprint?: string) =>
    request<List<Issue>>("GET", `/api/v1/projects/${seg(key)}/issues${sprint ? `?sprint=${seg(sprint)}` : ""}`).then((r) => r.items),
  rankIssue: (issueKey: string, anchor: { after: string } | { before: string }) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/rank`, anchor),
  createIssue: (key: string, title: string, type: IssueType, parentKey?: string) =>
    request<Issue>("POST", `/api/v1/projects/${seg(key)}/issues`, parentKey ? { title, type, parent_key: parentKey } : { title, type }),
  links: (issueKey: string) => request<List<IssueLink>>("GET", `/api/v1/issues/${seg(issueKey)}/links`).then((r) => r.items),
  link: (issueKey: string, type: LinkType, toKey: string) =>
    request<IssueLink>("POST", `/api/v1/issues/${seg(issueKey)}/links`, { type, to_key: toKey }),
  unlink: (id: string) => request<undefined>("DELETE", `/api/v1/links/${seg(id)}`),
  children: (issueKey: string) =>
    request<{ items: Issue[]; total: number; done: number; points: number; done_points: number }>("GET", `/api/v1/issues/${seg(issueKey)}/children`),
  worklogs: (issueKey: string) => request<List<Worklog>>("GET", `/api/v1/issues/${seg(issueKey)}/worklogs`).then((r) => r.items),
  logWork: (issueKey: string, spent: string, comment: string) =>
    request<Worklog>("POST", `/api/v1/issues/${seg(issueKey)}/worklogs`, { spent, comment }),
  deleteWorklog: (id: string) => request<undefined>("DELETE", `/api/v1/worklogs/${seg(id)}`),
  watchers: (issueKey: string) =>
    request<{ watching: boolean; items: { user_id: string; name: string }[] }>("GET", `/api/v1/issues/${seg(issueKey)}/watchers`),
  watch: (issueKey: string) => request<undefined>("POST", `/api/v1/issues/${seg(issueKey)}/watchers`),
  unwatch: (issueKey: string) => request<undefined>("DELETE", `/api/v1/issues/${seg(issueKey)}/watchers`),
  history: (issueKey: string) => request<List<HistoryEntry>>("GET", `/api/v1/issues/${seg(issueKey)}/history`).then((r) => r.items),
  workflow: (key: string) => request<Workflow>("GET", `/api/v1/projects/${seg(key)}/workflow`),
  setWorkflow: (key: string, wf: Workflow) => request<Workflow>("PUT", `/api/v1/projects/${seg(key)}/workflow`, wf),
  transition: (issueKey: string, to: Status) =>
    request<Issue>("POST", `/api/v1/issues/${seg(issueKey)}/transitions`, { to }),
  issue: (issueKey: string) => request<Issue>("GET", `/api/v1/issues/${seg(issueKey)}`),
  editIssue: (issueKey: string, patch: IssuePatch) => request<Issue>("PATCH", `/api/v1/issues/${seg(issueKey)}`, patch),
  comments: (issueKey: string) => request<List<Comment>>("GET", `/api/v1/issues/${seg(issueKey)}/comments`).then((r) => r.items),
  addComment: (issueKey: string, body: string) =>
    request<Comment>("POST", `/api/v1/issues/${seg(issueKey)}/comments`, { body }),
};
