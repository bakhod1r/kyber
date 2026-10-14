# JQL search

Kyber understands a Jira Query Language (JQL) dialect. `GET /api/v1/search?jql=…` searches every project
you may browse; issues hidden by security levels never match. Saved filters live at `/api/v1/filters`.

```text
project = KYB AND status != Done AND assignee = currentUser() ORDER BY priority DESC
labels IN (backend, api) AND sprint IN openSprints()
summary ~ "login" AND "story points" >= 3
duedate < 2026-11-01 AND resolution IS EMPTY
```

## Fields

| Field (aliases) | Kind |
|---|---|
| `project` | list |
| `key` (`issuekey`) | list (issue keys) |
| `type` (`issuetype`) | list |
| `status`, `statusCategory` | list |
| `priority` | list |
| `assignee`, `reporter` | list (users by email; `currentUser()`) |
| `sprint` | list (by name; `openSprints()`) |
| `fixVersion` | list |
| `labels` (`label`) | list |
| `resolution` | list |
| `parent` | list (issue keys) |
| `summary`, `description`, `text` | text |
| `startDate` (`"start date"`), `dueDate` (`"due date"`, `due`) | date, `YYYY-MM-DD` |
| `"story points"` (`storyPoints`, `points`) | number |

Custom fields can be queried by name as well; their type (text, number, date, select) decides the
operators.

## Operators

| Kind | Operators |
|---|---|
| list | `=` `!=` `IN (…)` `NOT IN (…)` `IS [NOT] EMPTY` |
| text | `~` (contains) `!~` `IS [NOT] EMPTY` |
| number, date | `=` `!=` `<` `<=` `>` `>=` `IS [NOT] EMPTY` |

- Combine clauses with `AND`, `OR`, `NOT` and parentheses.
- `EMPTY` and `NULL` are synonyms. Like Jira, `!=` and `NOT IN` never match empty fields.
- Functions: `currentUser()` (for `assignee`, `reporter`) and `openSprints()` (for `sprint`).
- `ORDER BY field [ASC|DESC], …` sorts by `key`, `project`, `priority`, `assignee`, `reporter`, `status`,
  `type`, `summary` or `points`; empty values come last.
- Quote values and multi-word field names with `"` or `'`. A query is at most 1000 characters.

An invalid query returns `422` with code `SEARCH_INVALID_JQL` and the position in the detail, e.g.
`JQL at 9: expected a value`.
