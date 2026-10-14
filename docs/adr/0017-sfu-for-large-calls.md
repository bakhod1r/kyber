# ADR-0017: A media server (SFU) for calls larger than six

Status: accepted (2026-10-14) — option B, LiveKit as an optional media server (product owner's choice)

## Context

Huddles (ADR-0009) are a WebRTC **full mesh**: every browser sends its audio/video to every other browser, and
Kyber relays only signaling over MQTT (ADR-0013). Upload grows with n − 1 streams (≈ 1.5 Mbit/s per video peer),
CPU grows with n − 1 encoders, so `domain.MaxParticipants = 6`. Teams want stand-ups and all-hands of 10–50 people.

A **Selective Forwarding Unit** (SFU) fixes this: each browser uploads once (with simulcast: three qualities),
the server forwards the right layer to each viewer. Upload stays constant; download grows only with what is
shown. Recording can also move to the server.

## Criteria

1. **Self-hosted, simple** — Kyber is one binary plus PostgreSQL; an SFU must not make a small install harder.
2. **Stack fit** — Go backend, DDD layers, archtest; the team maintains Go, not C++/Node/Rust.
3. **Scale** — 50 people in a call; several Kyber replicas.
4. **Licence** — compatible with Kyber's open-source licence (permissive preferred).
5. **Features** — simulcast, screen share, speaker detection, server-side recording, TURN-friendly.
6. **Security** — media never leaves the operator's servers; authorization stays in Kyber (calls context).

## Options

| | A. Embedded Pion SFU | B. LiveKit (external, optional) | C. mediasoup | D. Janus |
|---|---|---|---|---|
| Language | Go (library) | Go server, own SDKs | Node.js + C++ workers | C |
| Deployment | inside `kyber` binary | extra container (+ Redis for multi-node) | separate Node service | separate service |
| Licence | MIT | Apache-2.0 | ISC | GPL-3.0 |
| Simulcast / SVC | yes, we write the layer selection | yes, built in | yes | yes |
| Recording | we write it (Pion → Ogg/WebM) | Egress service (extra container) | we write it (FFmpeg) | plugin |
| Multi-replica | one call pinned to one replica (we route) | built in (Redis) | we build it | we build it |
| Client | plain browser WebRTC (our code) | LiveKit JS SDK | mediasoup-client | janus.js |
| Our code to write | most (SFU loop, bandwidth estimation, layers) | least (token minting, webhooks) | much + a Node service | much + C plugin ops |
| Fit with criteria 1–2 | best | good when optional | poor (Node) | poor (C, GPL) |

### A. Embedded Pion SFU
Pion (`github.com/pion/webrtc/v4`) is the de-facto Go WebRTC stack. A small SFU inside a new
`internal/call/infrastructure/sfu` adapter: one `PeerConnection` per participant, forward RTP tracks, pick
simulcast layers from receiver estimates (TWCC/REMB), PLI on layer switch. Media ports: one UDP mux port
(`KYBER_MEDIA_UDP`, e.g. 7882) plus TURN for the rest. Replicas: a call lives on the replica that started it;
others redirect signaling to it through MQTT (the call already knows its owner). Recording: write Opus/VP8
to Ogg/IVF and reuse the existing transcript pipeline.
+ zero extra services, Go, MIT, full control, auth stays in the calls context.
− we own congestion control and layer selection — the hard part of an SFU; a few weeks of careful work;
  quality at 50 people below LiveKit's tuned stack at first.

### B. LiveKit, optional
Kyber keeps the mesh for ≤ 6 people. When `KYBER_LIVEKIT_URL`, `_KEY`, `_SECRET` are set, larger calls go
to LiveKit: Kyber authorizes the user (calls context) and mints a short-lived room JWT; the browser uses
`livekit-client`; LiveKit webhooks (signed) update participants. Recording via LiveKit Egress.
+ production-grade SFU today (simulcast, dynacast, speaker detection, multi-node), little code for us.
− one more service to run (and Redis for clustering); a second client stack in the web app; key/secret to
  operate (environment only, never logged).

### C. mediasoup — excellent SFU, but a Node.js service beside a Go binary, and we would still write
signaling, scaling and recording. Rejected for stack fit.

### D. Janus — mature, but C, GPL-3.0 and plugin-based operations. Rejected for licence and stack fit.

## Decision

**B: LiveKit, optional, behind a port.** Chosen for "large calls working well soon": LiveKit already does
simulcast, dynacast, congestion control and multi-node, which option A would take weeks to match.

- The calls application layer has a `Rooms` port (`Ticket(room, user, now)`); `internal/call/infrastructure/livekit`
  implements it by signing a HS256 JWT with LiveKit's video grant (room join, publish, subscribe, data — never
  room admin) for one call's room, valid 15 minutes. No LiveKit SDK on the server; the secret stays in memory.
- With `KYBER_LIVEKIT_URL`/`_KEY`/`_SECRET` set, **every new call is a room** (`calls.room`, migration 0086), up
  to `MaxRoomParticipants = 50`; without them every call is a mesh (6), as before. A live call never switches
  mode (moving 6 browsers from mesh to SFU mid-call is fragile; an all-hands simply starts as a room).
- `POST /api/v1/calls/{id}/room` gives a participant `{url, token, expires_at}` (409 for a mesh call or a
  member not in the call). Mesh signaling is refused in a room.
- Kyber stays the source of truth for who is in the call (join, leave, media flags, hands, presentation,
  recording banner — unchanged); LiveKit only carries media. The web app keeps one UI: the huddle engine
  publishes the same tracks to the room instead of to peers; `livekit-client` loads lazily, only in room calls.
- The CSP's `connect-src` adds the LiveKit origin (and `worker-src blob:` for its client) only when configured.
- A pure-Go embedded SFU (option A) stays possible later as another adapter of the same port.

## Consequences

- One more service for operators who want large calls (`docker compose --profile rooms`); none for those who
  do not. Ops: `docs/ops/calls.md`.
- Leaving or being removed (the host may, S103) is a domain event, `call.participant_left`; its handler calls
  LiveKit's `RemoveParticipant` (Twirp, a token with `roomAdmin` for that room only), so nobody keeps listening
  after they are out of the call.
- Server-side recording (S104): with Egress and a shared directory configured, **Record** in a room call starts a
  room-composite MP4 egress (`roomRecord` grant) instead of the browser recorder; stopping it is the domain event
  `call.recording_stopped`, whose handler stops the egress, waits for the file (bounded; the outbox retries) and
  posts it to the conversation through the Chat context like a browser recording. Without Egress, rooms are recorded
  in the browser as before.
- CI runs the room e2e against a real `livekit-server --dev` (`make e2e-rooms`).
