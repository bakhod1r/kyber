# Sprint 13 — Finishing touches for chat and huddles

Source: product request (2026-10-14, "plan qil va boshla"): the items left open by Sprints 11–12. Planned first, then
built DDD + TDD (contract tests on memory and PostgreSQL, acceptance tests on every backend, Playwright e2e).

- ✅ **S82 Chat files on S3** — main now has an S3/MinIO blob adapter for issue attachments (`KYBER_S3_*`). Chat
  files and huddle recordings use the same store when it is configured (one bucket, a `chat/` prefix), so several
  replicas share them. AC: with S3 set, upload → download works on another replica; without it, unchanged.
  **Done:** `platform/blob/s3` (contract `blob/blobtest`, fake S3 in tests); `KYBER_S3_*` → chat files under
  `chat/` in the attachments bucket.
- ✅ **S83 One realtime stream** — the app opens two SSE streams (`/api/v1/stream` for chat/calls, main's
  `/api/v1/events` for boards). Serve main's events over the chat stream as well and let the web app open one.
  AC: one EventSource per tab; board live updates and chat both still arrive; `/api/v1/events` stays for
  compatibility.
  **Done:** `live.Hub.Follow` + realtime `Feed`s: `/api/v1/stream` also carries `change`/`resync` hints (same browse
  check); the web app's live updates listen on the shared stream — e2e shows no `/api/v1/events` request.
- ✅ **S84 Bot users with tokens** — main added personal access tokens. A workspace admin creates a bot (a user that
  cannot sign in) and gets a token for it; the bot posts to channels it was invited to through the normal API.
  AC: bots show with a 🤖 badge; a bot never gets notifications by email/Telegram; tokens shown once.
  **Done:** `GET/POST /api/v1/bots` (workspace owners/admins; site admins in the default workspace); Identity's
  `CreateBot` (no password, `…@bots.kyber.invalid`, a non-expiring PAT), migration 0069; 🤖 status; Notify never
  mails `.invalid` addresses (also fixes mail to Google/Telegram placeholder addresses). "🤖 Bots" in Activity and
  an Invite picker in channel settings. API 0.61.0.
- ✅ **S85 Drag to reorder sidebar sections** — my sections in my own order (drag a section header). AC: per user;
  new sections go last.
  **Done:** `GET/PUT /chat/sections` (migration 0070); drag a section header onto another (or ↑ for keyboards);
  unordered sections follow by name. API 0.62.0.
- **S86 Comments on whiteboard elements and doc lines** — select an element (or a doc paragraph) and comment;
  comments show as a pin; mentions notify. AC: only people who may read the board/doc see them.
  **Done (whiteboards, S86a):** `GET/POST /whiteboards/{id}/comments`, `DELETE /whiteboard-comments/{id}` (author
  only; migration 0071); 💬 count badges on elements, a comment panel for the selected element; @mentions notify
  like chat. API 0.63.0. Doc paragraphs (S86b) next.
- **S87 Recording summaries (optional)** — when an OpenAI-compatible chat model is configured (`KYBER_LLM_URL`),
  a recording's transcript also gets a short summary. Off by default; nothing leaves the server without it.

Order: S82 → S83 (infrastructure first), then S84, S85, S86, S87.
