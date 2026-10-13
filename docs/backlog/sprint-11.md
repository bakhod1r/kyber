# Sprint 11 — Team chat and huddles (video, audio, screen share)

Source: product request (2026-10-13). Design: ADR-0006. Order: **chat first, then video calls**. Every story is
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
    author edits/deletes; deleted messages show as "deleted" in history. ✅ **Done:** migration 0037, API 0.27.0.
- **S51 Realtime stream** — `GET /api/v1/stream` (SSE), in-process Hub port, events `message.*`,
  `conversation.*`, typing (`POST /conversations/{id}/typing`, ephemeral), presence (online while a stream is open).
  AC: a member receives a posted message within 1 s; non-members never receive it; reconnect resumes.
  ✅ **Done:** `platform/realtime` Hub (ring of 1024 events for `Last-Event-ID` replay, slow tabs are closed and
  reconnect), `GET /stream`, `GET /presence`, `POST /conversations/{id}/typing`, API 0.28.0. Multi-replica feed
  (LISTEN/NOTIFY) follows with S58.
- **S52 Threads, reactions, mentions, issue previews** — `thread_id` replies + reply count; reactions toggle
  (one per emoji per user); `@email`/`@name` mentions notify through the Notify context; issue keys in a message
  come back as previews only when the reader may browse the issue.
- **S53 Chat UI** — sidebar (channels, DMs, unread badges), channel view with infinite scroll, composer with
  mentions, thread panel, reactions, typing line; TanStack Query + the stream.
- **S54 Pins, search, files** (P1) — pin messages, search in messages I can read, file attachments via BlobStore.

## Video calls (huddles)

- **S55 Call context (backend)** — start/join/leave a huddle in a conversation I belong to; one live call per
  conversation; ≤ 6 participants; media state (audio/video/screen on/off); call ends when the last one leaves;
  "huddle started/ended" system messages in the conversation. AC: non-members get 404; 7th participant → 409.
- **S56 Signaling** — `POST /calls/{id}/signal` relays `offer|answer|candidate` to one participant over the
  stream; only participants may signal and receive; `GET /calls/ice` returns STUN/TURN servers with
  short-lived TURN credentials (`KYBER_ICE_SERVERS`, `KYBER_TURN_SECRET`).
- **S57 Huddle UI** — WebRTC mesh: camera, microphone, screen share (`getDisplayMedia`) with mute/camera/share
  toggles, participant grid, active-speaker highlight, device picker, join from channel header.
- **S59 Whiteboard (Zoom-style drawing)** — open a whiteboard in a huddle or channel; draw rectangles, ellipses,
  arrows, lines, freehand, text and sticky notes (e.g. an architecture diagram); everyone sees changes and
  cursors live; undo/redo; export PNG/SVG; sticky note → issue. AC: only conversation members open or edit it;
  two people moving the same shape end in one consistent state; it is kept after the call ends.
- **S60 Shared file in a call** — "present to all": everyone opens the same Markdown doc or whiteboard, edits it
  together and comments on a line/element; the presenter can follow-me (everyone scrolls with them).
- **S58 Ops** (P1) — coturn example in docker-compose/Helm docs, call metrics (`kyber_calls_active`), SFU ADR.
