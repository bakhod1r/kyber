// access: HTTP gateway to the access API (infrastructure).
import { request, seg } from "../../../shared/http";
import type { PermissionGrants, PermissionScheme } from "../domain/model";

export const accessGateway = {
  permissions: (key: string) => request<PermissionScheme>("GET", `/api/v1/projects/${seg(key)}/permissions`),
  setPermissions: (key: string, permissions: PermissionGrants[]) =>
    request<PermissionScheme>("PUT", `/api/v1/projects/${seg(key)}/permissions`, { permissions }),
  resetPermissions: (key: string) => request<undefined>("DELETE", `/api/v1/projects/${seg(key)}/permissions`),
};
