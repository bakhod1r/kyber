// docs: domain model — project pages and their history (Markdown is read by shared/markdown).
export type Page = {
  id: string;
  project_key: string;
  title: string;
  body: string;
  author_id: string;
  updated_by: string;
  updated_at: string;
  version: number;
};
export type PageInput = { title: string; body: string };
export type PageRevision = { number: number; title: string; body: string; by: string; at: string };
