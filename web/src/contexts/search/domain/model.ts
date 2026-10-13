// search: domain model — JQL search results and saved filters.
export type SearchHit = {
  key: string;
  project_key: string;
  title: string;
  type: string;
  status: string;
  status_name: string;
  status_category: string;
  priority: string;
  assignee_id: string | null;
  resolution: string;
  points: number | null;
};
export type SearchResult = { total: number; items: SearchHit[] };
export type Filter = { id: string; owner_id: string; name: string; jql: string; shared: boolean; version: number; mine: boolean };
export type FilterInput = { name: string; jql: string; shared: boolean };

/** Example queries offered on an empty search page. */
export const EXAMPLE_QUERIES = [
  "assignee = currentUser() AND resolution IS EMPTY ORDER BY priority DESC",
  "sprint in openSprints() AND statusCategory != done",
  'type = bug AND summary ~ "login"',
];
