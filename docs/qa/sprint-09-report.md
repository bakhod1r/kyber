# QA Report — Sprint 09

**Teams:** Quality (exploratory + suites), Architecture review, Backend review · **Verdict:** ✅ all confirmed bugs fixed with
regression tests; one high-severity item (per-workspace project keys) scheduled as the next story.

## Suites
`go test -race ./...` (memory, PostgreSQL, PostgreSQL+Redis) · 73 Vitest tests · 4 Playwright e2e · `make cover-strict`
(16 packages at 100 %) · OpenAPI contract/drift check on every response.

## Confirmed bugs (all fixed, each with a regression test)
| ID | Sev. | Finding | Fix |
|---|---|---|---|
| QA-2 | High | Burndown/velocity lost the scope when a sprint started right after planning (in-memory mode) | events carry the time they were written |
| QA-3 | Med | Export → import added a `'` before formula-like titles | import strips the export guard |
| QA-4 | Med | 100 000-character and multi-line titles; unbounded names | Jira summary rules (≤255, one line); names ≤100 |
| QA-5 | Med | Outsiders learned that a meeting exists (403) | 404 for outsiders, 403 for attendees |
| QA-6 | Low | Unknown routes / wrong methods answered text/plain | problem+json (6097/6098) |
| QA-7 | Low | Meetings visible on a workspace to non-members (`/api/v1/me` matched as a prefix of `/api/v1/meetings`) | exact allow-list |
| QA-8 | Low | An empty Reporter cell matched a member with an empty name (found while fixing QA-3) | empty cells never match |
| AR-1 | High | Notifications crossed workspaces | per-workspace inboxes, migration 0015 |
| BR-2 | High | A failed identity link locked a Telegram user out; concurrent first logins raced | recoverable linking, race handled, synthetic domain reserved |
| BR-3 | Med | Login CSRF on OTP endpoints | JSON required |
| BR-4 | Med | Telegram widget payload replayable for 24 h | 5 minutes |
| BR-5/6 | Med | OTP rows never purged; one slow Telegram update stalled others | purge job; 10 s per update |
| UI-1 | Med | Timer controls unusable while the issue panel was open (found by e2e) | top bar above the overlay |

Not reproduced (kept as regression tests): Google key fetch after the discovering request ended; panic text leaking to clients.

## Jira parity changes
Simplified workflow (any → any), Assignable User (no viewer assignees), Work On Issues (viewers cannot log focus time),
no meetings in the past, idempotent re-import of a project's own export.

## Open
- ~~QA-1 (High): project keys unique per installation~~ — fixed: keys are per workspace (migration 0017, ADR-0004 phase 2), acceptance test on all backends.
- Mentions use `@email` inserted by the autocomplete (Jira stores account ids); `@name` free text is not parsed.
