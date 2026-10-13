// admin: HTTP gateway to the administration API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { AdminMember, AdminProject, DeadLetter, Role } from "../domain/model";

export const adminGateway = {
  access: () => request<{ admin: boolean }>("GET", "/api/v1/admin/access").then((r) => r.admin),
  members: () => request<List<AdminMember>>("GET", "/api/v1/admin/members").then((r) => r.items),
  changeRole: (id: string, role: Role) => request<undefined>("PUT", `/api/v1/admin/members/${seg(id)}`, { role }),
  projects: () => request<List<AdminProject>>("GET", "/api/v1/admin/projects").then((r) => r.items),
  deadLetters: () => request<List<DeadLetter>>("GET", "/api/v1/admin/dead-letters").then((r) => r.items),
  retry: (id: number) => request<undefined>("POST", `/api/v1/admin/dead-letters/${id}/retry`),
};
