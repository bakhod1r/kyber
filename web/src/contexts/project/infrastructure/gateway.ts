// project: HTTP gateway to the project API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Member, Project, Role } from "../domain/model";

export const projectGateway = {
  projects: () => request<List<Project>>("GET", "/api/v1/projects").then((r) => r.items),
  createProject: (key: string, name: string) => request<Project>("POST", "/api/v1/projects", { key, name }),
  members: (key: string) => request<List<Member>>("GET", `/api/v1/projects/${seg(key)}/members`).then((r) => r.items),
  setMember: (key: string, email: string, role: Role) =>
    request<undefined>("POST", `/api/v1/projects/${seg(key)}/members`, { email, role }),

};
