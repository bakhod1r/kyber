// issue: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import type { Issue, IssueType } from "../domain/model";
import { issueGateway } from "../infrastructure/gateway";

export const issueUseCases = {
  ...issueGateway,
  /** Creates an issue; during an active sprint it joins that sprint (as in Jira), otherwise it
   * would land in the backlog and vanish from the sprint board. */
  async createIssueIn(projectKey: string, title: string, type: IssueType, sprintId?: string): Promise<Issue> {
    const created = await issueGateway.createIssue(projectKey, title, type);
    return sprintId ? issueGateway.editIssue(created.key, { version: created.version, sprint_id: sprintId }) : created;
  },
};
export * from "../domain/model";
