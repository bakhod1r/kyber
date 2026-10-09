# QA Report — Sprint 08

**Teams:** Quality + Security · **Verdict:** ✅ S29–S31 ready for **v0.8.0** · S32 (MCP server) carried into the next increment

## Acceptance criteria coverage
| Story | Evidence | Result |
|---|---|---|
| S29 Jira CSV import | parser on a realistic export (BOM, quoted multi-line cells, duplicate columns, header aliases, line numbers); mapping tables for type/status/priority incl. legacy Blocker…Trivial; `Plan` errors + warnings; `NewImportedIssue` emits exactly one `issue.imported`; Insights replays it at original created/resolved times (redelivery-safe); mapping repo contract on memory **and** PostgreSQL; service tests (preview is side-effect free, idempotent re-run, stop-at-failure then resume, admin/outsider/size/invalid-file guards); `TestS29JiraImport` ×3 backends incl. **no notifications** to the matched assignee; 3 UI tests | PASS |
| S30 CSV export | `WriteJiraCSV` → `ParseJiraCSV` → `Plan` round trip (commas, quotes, newlines, sub-task, points); `TestS30CSVExport` ×3: export from KYB, import into NEW, every field equal; viewer may export, outsider 404; UI link test | PASS |
| S31 @mention autocomplete | 4 component tests: filter by name/email, ArrowUp/Down + `aria-activedescendant`, Enter/Tab/click insert `@email `, Escape dismisses, emails in prose do not trigger; e2e mention flow still green | PASS |

## Security review
| Check | Result |
|---|---|
| AuthZ | import = project **admin**; export = any member (viewer included); non-members get 404 (no existence leak) |
| Input limits | 10 MB CSV enforced in the service; HTTP body capped at 2× + 1 KB for JSON escaping → 413 `IMPORT_TOO_LARGE` |
| CSV injection (export) | cells starting with `= + - @ TAB CR` are prefixed with `'` — verified by unit and acceptance tests |
| Notification flood | a single `issue.imported` event; notify does not subscribe to it |
| Untrusted file content | parsed with `encoding/csv` (`LazyQuotes` on to tolerate real Jira exports), never evaluated or executed; errors report line numbers only |

## Found & fixed this sprint
| ID | Severity | Finding | Fix |
|---|---|---|---|
| QA-08-1 | Low | Row line numbers were off for rows after a multi-line description | `csv.Reader.FieldPos` for the record's first line |
| QA-08-2 | Low | A BOM literal in Go test source is rejected by the compiler | byte escape `\xef\xbb\xbf` |

## Known limitations (accepted)
- The per-project import lock is in-process: two replicas importing the **same file at the same moment** could
  both create an issue for one row (the mapping insert of the second fails). Follow-up: a Postgres advisory lock.
- The issue's own `created_at` column is the import time; reports and charts use the original Jira dates.
- Jira comments, attachments, sprints and links are not imported yet (CSV export does not carry them reliably).
