// automation: HTTP gateway to the automation API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Rule, RuleInput, Run } from "../domain/model";

export const automationGateway = {
  rules: (key: string) => request<List<Rule>>("GET", `/api/v1/projects/${seg(key)}/automation/rules`).then((r) => r.items),
  runs: (key: string) => request<List<Run>>("GET", `/api/v1/projects/${seg(key)}/automation/runs`).then((r) => r.items),
  createRule: (key: string, r: RuleInput) => request<Rule>("POST", `/api/v1/projects/${seg(key)}/automation/rules`, r),
  updateRule: (r: Rule, enabled: boolean) =>
    request<Rule>("PUT", `/api/v1/automation/rules/${seg(r.id)}`, {
      version: r.version, enabled, name: r.name, trigger: r.trigger, to_status: r.to_status, condition: r.condition, actions: r.actions,
    }),
  deleteRule: (id: string) => request<undefined>("DELETE", `/api/v1/automation/rules/${seg(id)}`),
};
