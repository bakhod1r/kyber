// focus: HTTP gateway to the focus API (infrastructure).
import { request, seg } from "../../../shared/http";
import type { FocusSession } from "../domain/model";

export const focusGateway = {
  focus: () => request<{ session: FocusSession | null }>("GET", "/api/v1/focus").then((r) => r.session),
  startFocus: (issueKey: string, minutes: number) => request<FocusSession>("POST", "/api/v1/focus", { issue_key: issueKey, minutes }),
  focusAction: (action: "pause" | "resume" | "stop") => request<FocusSession>("POST", `/api/v1/focus/${action}`),
  issueFocus: (issueKey: string) =>
    request<{ items: FocusSession[]; total_focused_seconds: number }>("GET", `/api/v1/issues/${seg(issueKey)}/focus`),
};
