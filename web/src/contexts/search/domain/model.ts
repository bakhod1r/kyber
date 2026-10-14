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

/** Fields JQL understands, in the order autocomplete offers them. */
export const JQL_FIELDS = [
  "project", "type", "status", "statusCategory", "priority", "assignee", "reporter", "sprint", "fixVersion", "labels",
  "resolution", "parent", "summary", "description", "text", "startDate", "dueDate", '"story points"',
];
const OPERATORS = ["=", "!=", "in", "not in", "is", "is not", "~", "!~", ">", ">=", "<", "<="];
const KEYWORDS = ["AND", "OR", "ORDER BY"];
const VALUES: Record<string, string[]> = {
  type: ["epic", "story", "task", "bug", "subtask"],
  status: ["todo", "in_progress", "done"],
  statuscategory: ["todo", "in_progress", "done"],
  priority: ["highest", "high", "medium", "low", "lowest"],
  resolution: ["done", "wont_do", "duplicate", "cannot_reproduce", "unresolved"],
  assignee: ["currentUser()", "EMPTY"],
  reporter: ["currentUser()"],
  sprint: ["openSprints()", "EMPTY"],
};

/** Values autocomplete cannot know statically, by lower-case field (e.g. labels, project keys). */
export type JqlValues = Partial<Record<string, string[]>>;
export type JqlCompletion = { from: number; options: string[]; apply: (option: string) => string };

/** completeJql suggests what can follow the text (Jira's JQL autocomplete): fields, operators,
 * values of the field being compared, or AND / OR / ORDER BY after a complete clause. */
export function completeJql(text: string, extra: JqlValues): JqlCompletion {
  const tokens = [...text.matchAll(/"[^"]*"?|[(),]|!=|>=|<=|!~|[=<>~]|[^\s(),=<>!~"]+/g)].map((m) => ({ s: m[0], at: m.index ?? 0 }));
  const last = tokens[tokens.length - 1];
  const open = last && last.at + last.s.length === text.length && !/^[(),=<>!~]+$/.test(last.s);
  const from = open ? last.at : text.length;
  const prefix = open ? last.s.replace(/^"/, "").toLowerCase() : "";
  const prev = (open ? tokens.slice(0, -1) : tokens).map((t) => t.s.toLowerCase());
  const options = candidates(prev, extra).filter((o) => o.replace(/^"/, "").toLowerCase().startsWith(prefix) && o.toLowerCase() !== prefix);
  return { from, options: options.slice(0, 10), apply: (o) => `${text.slice(0, from)}${o} ` };
}

const isField = (t: string | undefined) => !!t && JQL_FIELDS.some((f) => f.toLowerCase() === t);

function valuesOf(field: string | undefined, extra: JqlValues): string[] {
  const f = (field ?? "").replace(/"/g, "");
  return [...(extra[f] ?? []), ...(VALUES[f] ?? [])];
}

function candidates(prev: string[], extra: JqlValues): string[] {
  const n = prev.length;
  const at = (k: number) => prev[n - k];
  // Inside an unclosed "in (...)" list: values of that field.
  const paren = prev.lastIndexOf("(");
  if (paren >= 0 && !prev.slice(paren).includes(")") && prev[paren - 1] === "in") {
    return valuesOf(prev[paren - 1 - (prev[paren - 2] === "not" ? 2 : 1)], extra);
  }
  if (n === 0 || ["and", "or", "not", "("].includes(at(1)!) || (at(1) === "by" && at(2) === "order")) return JQL_FIELDS;
  if (at(1) === "is" || (at(1) === "not" && at(2) === "is")) return ["EMPTY"];
  if (isField(at(1))) return OPERATORS;
  if (["=", "!=", "~", "!~", ">", ">=", "<", "<="].includes(at(1)!)) return valuesOf(at(2), extra);
  if (at(1) === "in") return ["("];
  return KEYWORDS;
}

/** The project a query is about (`project = KEY`), for project-specific suggestions. */
export function queryProject(jql: string): string {
  return /\bproject\s*=\s*"?([A-Za-z][A-Za-z0-9_]*)/i.exec(jql)?.[1]?.toUpperCase() ?? "";
}
