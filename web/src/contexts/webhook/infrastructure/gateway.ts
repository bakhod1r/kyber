// webhook: HTTP gateway to the webhooks and development APIs (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { DevelopmentLink, Webhook, WebhookDelivery, WebhookInput } from "../domain/model";

const base = (key: string) => `/api/v1/projects/${seg(key)}/webhooks`;

export const webhookGateway = {
  webhooks: (key: string) => request<List<Webhook>>("GET", base(key)).then((r) => r.items),
  createWebhook: (key: string, w: WebhookInput) => request<Webhook>("POST", base(key), w),
  updateWebhook: (key: string, id: string, w: WebhookInput) => request<Webhook>("PUT", `${base(key)}/${seg(id)}`, w),
  deleteWebhook: (key: string, id: string) => request<undefined>("DELETE", `${base(key)}/${seg(id)}`),
  deliveries: (key: string, id: string) => request<List<WebhookDelivery>>("GET", `${base(key)}/${seg(id)}/deliveries`).then((r) => r.items),
  development: (issueKey: string) => request<List<DevelopmentLink>>("GET", `/api/v1/issues/${seg(issueKey)}/development`).then((r) => r.items),
};
