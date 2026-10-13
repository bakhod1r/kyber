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

/** Quick filters offered on the board and the list (Jira's quick filters). */
export const QUICK_FILTERS: { id: string; label: string; jql: string }[] = [
  { id: "mine", label: "Only my issues", jql: "assignee = currentUser()" },
  { id: "unassigned", label: "Unassigned", jql: "assignee is EMPTY" },
  { id: "open", label: "Not done", jql: "statusCategory != done" },
  { id: "bugs", label: "Bugs", jql: "type = bug" },
];

/** The part of a JQL query before its ORDER BY. */
export function withoutOrder(jql: string): string {
  return jql.split(/\border\s+by\b/i)[0]?.trim() ?? "";
}

/** Builds the query of a project view: the project, the chosen quick filters (all must hold) and an
 * optional saved filter, then the order. */
export function projectQuery(project: string, quick: string[], saved = "", order = ""): string {
  const parts = [`project = "${project}"`];
  for (const id of quick) {
    const q = QUICK_FILTERS.find((f) => f.id === id);
    if (q) parts.push(`(${q.jql})`);
  }
  const s = withoutOrder(saved);
  if (s) parts.push(`(${s})`);
  return parts.join(" AND ") + (order ? ` ORDER BY ${order}` : "");
}
