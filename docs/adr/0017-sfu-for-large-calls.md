# ADR-0017: A media server (SFU) for calls larger than six

Status: proposed (2026-10-14) — awaiting the product owner's choice of option

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

## Recommendation

**A as the default, behind a port; B as an optional adapter later.** The calls application layer gets a
`MediaRouter` port ("join this participant, return an SDP answer / room token"). The mesh stays for small
calls; above six (or when the operator sets `KYBER_CALLS_SFU=always`) the call uses the embedded Pion SFU. Because
it is a port, a LiveKit adapter can be added for operators who already run LiveKit, without touching the domain.

If the priority is "large calls working well soon" over "one binary", choose **B** first instead.

## Consequences

- `MaxParticipants` becomes a per-mode limit (mesh 6, SFU configurable, default 50).
- New config: `KYBER_MEDIA_UDP` (port), `KYBER_MEDIA_PUBLIC_IP` (for ICE candidates behind NAT); the
  docker-compose and Helm chart open one UDP port. TURN (coturn) stays for strict networks.
- Signaling messages gain an `sfu` offer/answer pair; the web call context gets an SFU transport next to
  the mesh one (same UI).
- Server CPU/bandwidth now matter: ~0.5 core and ~1.5 Mbit/s × viewers per 10 video participants;
  documented in `docs/ops/calls.md`, with Prometheus metrics (tracks, bitrate, layer switches).
- Server-side recording becomes possible (later sprint); browser recording stays for mesh calls.

## Plan (if A is chosen) — Sprint 16
- S99 `MediaRouter` port, mode selection, per-mode limit (domain + application, TDD).
- S100 Pion SFU adapter: join/leave, forward audio+video, renegotiation; integration test with two Pion clients.
- S101 Simulcast and layer selection; speaker detection (audio levels).
- S102 Web SFU transport (one upload, many tracks), e2e with 8 fake participants.
- S103 Replicas: route a call's signaling to its owner replica; ops docs, Helm/compose UDP port, metrics.
