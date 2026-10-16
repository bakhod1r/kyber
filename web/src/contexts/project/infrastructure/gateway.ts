// project: HTTP gateway to the project API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { ProjectTemplate, Invitation, InvitationPreview, InviteResult, Member, Project, Role } from "../domain/model";

export const projectGateway = {
  projects: () => request<List<Project>>("GET", "/api/v1/projects").then((r) => r.items),
  createProject: (key: string, name: string, template?: ProjectTemplate) =>
    request<Project>("POST", "/api/v1/projects", template && template !== "kanban" ? { key, name, template } : { key, name }),
  archive: (key: string) => request<Project>("POST", `/api/v1/projects/${seg(key)}/archive`),
  restore: (key: string) => request<Project>("POST", `/api/v1/projects/${seg(key)}/restore`),
  members: (key: string) => request<List<Member>>("GET", `/api/v1/projects/${seg(key)}/members`).then((r) => r.items),
  setMember: (key: string, email: string, role: Role) =>
    request<undefined>("POST", `/api/v1/projects/${seg(key)}/members`, { email, role }),
  invite: (key: string, email: string, role: Role) => request<InviteResult>("POST", `/api/v1/projects/${seg(key)}/invitations`, { email, role }),
  invitations: (key: string) => request<List<Invitation>>("GET", `/api/v1/projects/${seg(key)}/invitations`).then((r) => r.items),
  revokeInvitation: (key: string, id: string) => request<undefined>("DELETE", `/api/v1/projects/${seg(key)}/invitations/${seg(id)}`),
  invitation: (token: string) => request<InvitationPreview>("GET", `/api/v1/invitations/${seg(token)}`),
  acceptInvitation: (token: string) => request<{ project: string }>("POST", `/api/v1/invitations/${seg(token)}/accept`),

};
