// announcement: domain model — news to a project's or a workspace's members.
export type Announcement = {
  id: string;
  project_key: string | null; // null = the whole workspace
  title: string;
  body: string;
  author_id: string;
  created_at: string;
};
export type AnnouncementInput = { title: string; body: string };
