# Sprint 11 — Team chat and huddles (video, audio, screen share)

Source: product request (2026-10-13). Design: ADR-0009. Order: **chat first, then video calls**. Every story is
planned before it starts (scope + acceptance criteria below), then built DDD + TDD with acceptance tests on all
backends.

## Chat

- **S50 Chat core (backend)** — Chat bounded context.
  - Channels: create (name `[a-z0-9-_]{1,80}`, unique per workspace, public or private, topic), list the ones I
    can see, join a public one, leave, invite a member to a private one.
  - DMs: open a DM / group DM with 1–7 workspace members; opening the same set again returns the same one.
  - Messages: post (1–4000 chars), edit and delete my own, history newest-first with `before` cursor paging.
  - Unread: mark read; the conversation list shows `unread` per conversation.
  - AC: non-members get 404 for private channels and DMs; outsiders of the workspace see nothing; only the
    author edits/deletes; deleted messages show as "deleted" in history. ✅ **Done:** migration 0041, API 0.27.0.
- **S51 Realtime stream** — `GET /api/v1/stream` (SSE), in-process Hub port, events `message.*`,
  `conversation.*`, typing (`POST /conversations/{id}/typing`, ephemeral), presence (online while a stream is open).
  AC: a member receives a posted message within 1 s; non-members never receive it; reconnect resumes.
  ✅ **Done:** `platform/realtime` Hub (ring of 1024 events for `Last-Event-ID` replay, slow tabs are closed and
  reconnect), `GET /stream`, `GET /presence`, `POST /conversations/{id}/typing`, API 0.28.0. Multi-replica feed
  (LISTEN/NOTIFY) follows with S58.
- **S52 Threads, reactions, mentions, issue previews** — `thread_id` replies + reply count; reactions toggle
  (one per emoji per user); `@email`/`@name` mentions notify through the Notify context; issue keys in a message
  come back as previews only when the reader may browse the issue.
  ✅ **Threads and reactions done** (replies stay out of channel history and unread, `reply_count`, toggle
  reactions, `message.reacted` on the stream; API 0.29.0).
  ✅ **Mentions done (S52b):** `chat.message_posted` goes through the outbox; Notify tells `@email`-mentioned people who
  may read the conversation (inbox + email/Telegram copies, link to `/chat/{id}`); migration 0042, API 0.31.0.
  ✅ **Issue previews done (S52c):** up to 5 keys per message, resolved per reader through Issue Tracking's
  authorized read (hidden issues are not previewed); `issues` on every REST message.
- **S53 Chat UI** — sidebar (channels, DMs, unread badges), channel view with infinite scroll, composer with
  mentions, thread panel, reactions, typing line; TanStack Query + the stream.
  ✅ **Done:** `web/src/contexts/chat` (four layers), `/chat` and `/chat/:id`, live updates over `EventSource`,
  unit tests and an e2e test with two browsers (message, reaction and typing arrive live).
- **S54 Pins, search, files** (P1) — pin messages, search in messages I can read, file attachments via BlobStore.

## Video calls (huddles)

- **S55 Call context (backend)** — start/join/leave a huddle in a conversation I belong to; one live call per
  conversation; ≤ 6 participants; media state (audio/video/screen on/off); call ends when the last one leaves;
  "huddle started/ended" system messages in the conversation. AC: non-members get 404; 7th participant → 409.
  ✅ **Done** (except the system messages): Call context, optimistic locking with retries so racing joins never
  exceed 6, one live call per conversation (unique index), one screen share at a time, `call.updated` on the
  stream; migration 0043, API 0.32.0.
  ✅ **System messages done:** `call.started`/`call.ended` go through the outbox; Chat writes "started a huddle" and
  "the huddle ended after N min" into the conversation (idempotent by ref); API 0.38.0.
- **S56 Signaling** — `POST /calls/{id}/signal` relays `offer|answer|candidate` to one participant over the
  stream; only participants may signal and receive; `GET /calls/ice` returns STUN/TURN servers with
  short-lived TURN credentials (`KYBER_ICE_SERVERS`, `KYBER_TURN_SECRET`).
  ✅ **Done:** API 0.33.0.
- **S57 Huddle UI** — WebRTC mesh: camera, microphone, screen share (`getDisplayMedia`) with mute/camera/share
  toggles, participant grid, active-speaker highlight, device picker, join from channel header.
  ✅ **Done:** `web/src/contexts/call` (HuddleEngine: mesh, perfect negotiation, the earlier participant opens each
  connection, signals queued per peer, ICE-restart watchdog), one shared SSE stream for all contexts, e2e in real
  Chromium with fake devices (camera both ways + screen share), 30/30 runs green.
- **S59 Whiteboard (Zoom-style drawing)** — open a whiteboard in a huddle or channel; draw rectangles, ellipses,
  arrows, lines, freehand, text and sticky notes (e.g. an architecture diagram); everyone sees changes and
  cursors live; undo/redo; export PNG/SVG; sticky note → issue. AC: only conversation members open or edit it;
  two people moving the same shape end in one consistent state; it is kept after the call ends.
  ✅ **Done:** Whiteboard context (per-element versions, atomic batches, reload-and-reapply), migration 0047, API
  0.36.0; SVG canvas with select/move, rectangle, ellipse, arrow, line, pen, text, sticky notes, colors, undo/redo,
  live changes and named cursors, SVG/PNG export; e2e with two browsers drawing together. Sticky note → issue: next.
- **S60 Shared file in a call** — "present to all": everyone opens the same Markdown doc or whiteboard, edits it
  together and comments on a line/element; the presenter can follow-me (everyone scrolls with them).
- **S58 Ops** (P1) ✅ **Done:** realtime fan-out across replicas (NOTIFY + `realtime_spill` for large events,
  migration 0051), gauges `kyber_stream_connections` / `kyber_calls_active`, coturn in docker-compose
  (`--profile calls`) and `docs/ops/calls.md`. SFU ADR: later.
  Original scope: — coturn example in docker-compose/Helm docs, call metrics (`kyber_calls_active`), SFU ADR.
