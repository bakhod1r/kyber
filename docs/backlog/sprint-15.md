# Sprint 15 — Finishing the collaboration features

Source: the items left after Sprint 14 (2026-10-14, "go"). Planned first, then built DDD + TDD.

- ✅ **S94 Chat, huddles and whiteboards in Uzbek and Russian** — the UI added in Sprints 11–14 was English only.
  AC: every visible string, label, title and placeholder in chat, huddles, whiteboards and the conversation header
  goes through `t()`; uz and ru translate every key (tsc enforces it); the other 23 languages fall back to English;
  tests keep their English queries. Dictionaries live per context in `web/src/shared/i18n/parts/`.
  **Done:** 282 keys (chat 158, huddles and conversation header 52, whiteboards 72) in uz and ru. Not translated:
  user data, emoji-only buttons, and labels produced in the chat application layer (status presets, reminder
  presets, slash-command help, section names) — a follow-up.
- ✅ **S95 Doc comment notifications open the page** — a `doc` notification links to the commented page itself, not
  the project's docs list. AC: email/Telegram link and the bell both open `…/docs?page=<id>`.
  **Done:** notifications keep `page_id` (migration 0082, API 0.68.0); Docs opens `?page=`.
- ✅ **S96 Images on whiteboards** — paste or upload an image onto a board (stored like chat files), move and resize it.
  **Done:** element kind `image` (migration 0083) whose text is the picture's id; `POST/GET /whiteboards/{id}/images`
  (members only; PNG/JPEG/GIF/WebP by their bytes, ≤ 10 MB, SVG refused; served with `nosniff` and a sandbox CSP;
  API 0.69.0); stored in the chat file store (disk, memory or S3). 🖼 button, paste and drag-and-drop place it fitted
  in the middle of the view. Also: the south-east resize handle is drawn on top, so tiny elements stay resizable.
- ✅ **S97 The rest of chat in Uzbek and Russian** — the labels the chat domain produces in English: status
  presets, "clear after" and reminder choices, the Channels / Direct messages sections and the slash-command help.
  **Done:** translated where they are shown (the domain keeps its English); a preset you pick is set in your
  language; sections you name yourself stay as you named them.
- ✅ **S98 README screenshots** — chat, huddle, whiteboard and docs, taken from the running app by a script.
  **Done:** `make screenshots` seeds a styled whiteboard (through the ops API) and a doc with paragraph comments;
  all 12 README images regenerated, plus a new Docs row.

Order: S94, then S95, S96, S97, S98.
