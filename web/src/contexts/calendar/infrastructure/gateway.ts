// calendar: HTTP gateway to the calendar API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Meeting } from "../domain/model";

export const calendarGateway = {
  meetings: (from?: string, to?: string) =>
    request<List<Meeting>>("GET", `/api/v1/meetings${from && to ? `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` : ""}`).then((r) => r.items),
  scheduleMeeting: (m: { title: string; starts_at: string; ends_at: string; attendee_ids: string[] }) =>
    request<Meeting>("POST", "/api/v1/meetings", m),
  cancelMeeting: (id: string) => request<undefined>("DELETE", `/api/v1/meetings/${seg(id)}`),
};
