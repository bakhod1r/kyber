# ADR-0007 — Team chat and calls (Slack-style messaging, huddles with video, audio and screen share)

- **Status:** Accepted (2026-10-13), implementation in Sprint 11 (S50–S58)
- **Related:** ADR-0005 (four DDD layers), ADR-0003 (access control), ADR-0004 (workspaces), PLAN §3.5 (bounded contexts)

## Context
Teams using Kyber switch to Slack/Zoom to talk about the work. We bring the parts of Slack that matter for a
tracker into the same self-hosted binary: channels, direct messages, threads, reactions, mentions, unread
counts, issue-key previews — and huddles (calls) with camera, microphone and screen sharing. No third-party
SaaS: everything runs on the operator's server.

## What we take from Slack (and what we do not)
| Slack feature | Kyber v1 | Later |
|---|---|---|
| Public / private channels, join / leave, topic | ✅ S50 | archive, rename, convert |
| Direct messages, group DMs (≤ 8 people) | ✅ S50 | |
| Post / edit / delete own message, history paging | ✅ S50 | |
| Unread counts, last-read cursor | ✅ S50 | |
| Realtime delivery, typing indicator, presence | ✅ S51 | |
| Threads (replies to a message, reply count) | ✅ S52 | "also send to channel" |
| Emoji reactions | ✅ S52 | custom emoji |
| `@mention` → notification (existing Notify context) | ✅ S52 | `@channel`, `@here` |
| Issue key preview (`KYB-12` unfurled, only if the reader may see it) | ✅ S52 | |
| Web UI (sidebar, channel view, composer, threads) | ✅ S53 | |
| Pins, search, file uploads (via attachment BlobStore) | S54 | |
| **Huddles**: audio, video, screen share in a channel or DM | ✅ S55–S57 | recording, SFU |
| **Whiteboard** in a huddle (Zoom-style): draw shapes, arrows, sticky notes, text — e.g. an architecture diagram | ✅ S59 | templates, export to issue |
| **Shared file** in a huddle: everyone opens the same doc/diagram, edits and comments on it live | ✅ S60 | version history |
| Workflows, bots, apps, Slack Connect, statuses with emoji | — | out of scope |

## Decision
1. **Two new bounded contexts.**
   - **Chat** (`internal/chat`): aggregates `Conversation` (kind `channel` | `dm`, private flag, members) and
     `Message` (author, body, thread parent, edited/deleted, reactions). Cross-aggregate by ID only. A member's
     read cursor (`last_read_at`) lives on the membership.
   - **Call** (`internal/call`): aggregate `Call` (conversation id, participants with joined/left, media state
     `audio|video|screen`), at most one live call per conversation. The call ends when the last participant
     leaves. Chat sees a call only through events ("huddle started" system message).
2. **Tenant and access.** Every conversation belongs to a workspace (ADR-0004); only workspace members can
   create or join. Public channels are readable by every workspace member; private channels and DMs only by
   their members. Outsiders get 404, never 403 (no existence leak, same as projects).
3. **Realtime transport = Server-Sent Events + HTTP POST**, not WebSocket. One `GET /api/v1/stream`
   (`text/event-stream`) per browser tab carries chat events, typing, presence and call signaling; the client
   sends with ordinary POSTs. Reasons: stdlib only (no new dependency), works through the existing auth, CSRF,
   tenant and audit middleware unchanged, survives proxies, auto-reconnects (`Last-Event-ID`). Fan-out is an
   in-process `Hub` behind a port; a Postgres `LISTEN/NOTIFY` (or Redis pub/sub) adapter makes it work across
   replicas (S51).
4. **Media = WebRTC.** Browsers send audio/video/screen directly to each other; the server only relays
   signaling (SDP offers/answers, ICE candidates) to participants of the same call over the stream above.
   - v1 topology: **full mesh**, limited to **6 participants** per call (domain invariant). Enough for a team
     huddle, zero media servers to operate.
   - Screen share: `getDisplayMedia()` adds a third track (`screen`); renegotiation over the same signaling.
   - NAT traversal: STUN/TURN servers come from config (`KYBER_ICE_SERVERS`); the docs ship a `coturn` example.
     TURN credentials are short-lived (HMAC, coturn `use-auth-secret`) and issued by `GET /api/v1/calls/ice`.
   - Later (ADR to follow): an SFU (LiveKit or pion) behind the same signaling port for >6 people and recording.
5. **Persistence.** Hand-written pgx repositories (ADR-0002) with the shared contract tests; migrations
   `0044_chat`, `0046_calls`. Signaling messages are never stored.
6. **Events.** Chat writes `message.posted` (and edited/deleted) to the outbox in the same transaction; Notify
   consumes it for mentions; the realtime Hub is fed from the app layer after commit.

7. **Whiteboard and shared files (S59–S60).** A `Board` (whiteboard) aggregate in the Call context, also
   attachable to a channel so it outlives the call. The canvas is a list of elements (`rect`, `ellipse`,
   `arrow`, `line`, `freehand`, `text`, `sticky`, `image`) with id, position, style and a per-element version.
   Edits are small operations (`add`, `move`, `update`, `delete`) sent by POST and fanned out over the stream;
   conflicts resolve last-writer-wins per element (version check), which is enough for whiteboards and needs no
   CRDT library. Live cursors and "who is drawing" are ephemeral stream events. A shared file is a Markdown
   document or a whiteboard opened by everyone in the call ("present to all"), with comments anchored to a
   line or element. Export: PNG/SVG, and "create issue from sticky note". Frontend draws on an HTML canvas
   (no external SaaS).

## Consequences
- No new Go dependency for v1; the frontend uses only browser APIs (`EventSource`, `RTCPeerConnection`).
- Mesh costs upload bandwidth per participant (n−1 streams); the 6-person cap keeps it acceptable.
- SSE is one-way, so every client action is a POST — fine for chat, and signaling volume is small.
