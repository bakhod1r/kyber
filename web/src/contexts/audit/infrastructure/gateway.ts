// audit: HTTP gateway to the audit API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { AuditEntry } from "../domain/model";

export const auditGateway = {
  audit: (key: string) => request<List<AuditEntry>>("GET", `/api/v1/projects/${seg(key)}/audit`).then((r) => r.items),
};
