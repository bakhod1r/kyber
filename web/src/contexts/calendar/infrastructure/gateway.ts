// calendar: HTTP gateway to the calendar API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Meeting, MeetingChange, MeetingInput, MeetingResponse } from "../domain/model";

export const calendarGateway = {
  meetings: (from?: string, to?: string) =>
    request<List<Meeting>>("GET", `/api/v1/meetings${from && to ? `?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}` : ""}`).then((r) => r.items),
  scheduleMeeting: (m: MeetingInput) => request<Meeting>("POST", "/api/v1/meetings", m),
  rescheduleMeeting: (id: string, c: MeetingChange) => request<Meeting>("PUT", `/api/v1/meetings/${seg(id)}`, c),
  cancelMeeting: (id: string) => request<undefined>("DELETE", `/api/v1/meetings/${seg(id)}`),
  respond: (id: string, response: MeetingResponse) => request<Meeting>("PUT", `/api/v1/meetings/${seg(id)}/response`, { response }),
};
