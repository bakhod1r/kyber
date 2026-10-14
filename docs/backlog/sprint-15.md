# Sprint 15 — Finishing the collaboration features

Source: the items left after Sprint 14 (2026-10-14, "go"). Planned first, then built DDD + TDD.

- **S94 Chat, huddles and whiteboards in Uzbek and Russian** — the UI added in Sprints 11–14 was English only.
  AC: every visible string, label, title and placeholder in chat, huddles, whiteboards and the conversation header
  goes through `t()`; uz and ru translate every key (tsc enforces it); the other 23 languages fall back to English;
  tests keep their English queries. Dictionaries live per context in `web/src/shared/i18n/parts/`.
- **S95 Doc comment notifications open the page** — a `doc` notification links to the commented page itself, not
  the project's docs list. AC: email/Telegram link and the bell both open `…/docs?page=<id>`.
- **S96 Images on whiteboards** — paste or upload an image onto a board (stored like chat files), move and resize it.

Order: S94, then S95, S96.
