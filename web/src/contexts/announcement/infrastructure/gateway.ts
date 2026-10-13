// announcement: HTTP gateway (infrastructure). A project key of null addresses the workspace.
import { request, seg, type List } from "../../../shared/http";
import type { Announcement, AnnouncementInput } from "../domain/model";

const path = (project: string | null) => (project ? `/api/v1/projects/${seg(project)}/announcements` : "/api/v1/announcements");

export const announcementGateway = {
  announcements: (project: string | null) => request<List<Announcement>>("GET", path(project)).then((r) => r.items),
  publish: (project: string | null, a: AnnouncementInput) => request<Announcement>("POST", path(project), a),
};
