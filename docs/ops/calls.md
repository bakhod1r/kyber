# Running chat and huddles in production

Chat, huddles (audio, video, screen share), whiteboards and "present to all" need nothing beyond Kyber and
PostgreSQL — except that video calls between people on different networks need a STUN/TURN server.
Design: [ADR-0009](../adr/0009-chat-and-calls.md).

## How media flows

Kyber never touches audio or video. Browsers connect to each other (WebRTC, a full mesh of at most 6 people);
Kyber only relays the signaling (who wants to talk to whom) over its event stream.

| Situation | What is needed |
|---|---|
| Everyone on the same LAN / VPN | nothing (`KYBER_ICE_SERVERS` may stay empty) |
| People on different networks, ordinary NAT | a **STUN** server (finds each browser's public address) |
| Strict NAT, corporate firewalls, mobile carriers | a **TURN** server (relays the media) — about 10–20 % of connections |

## coturn

`docker-compose.yml` has a `coturn` service behind the `calls` profile:

```sh
docker compose --profile calls up -d
```

1. Choose a long random secret and set it both as coturn's `--static-auth-secret` and as `KYBER_TURN_SECRET`.
   Kyber hands each browser TURN credentials valid for 12 hours (`username = expiry:user`,
   `credential = base64(HMAC-SHA1(secret, username))`, coturn's `use-auth-secret` scheme) — the secret itself
   never leaves the server and never appears in logs.
2. Point `KYBER_ICE_SERVERS` at the host, e.g.
   `stun:turn.example.com:3478,turn:turn.example.com:3478?transport=udp,turn:turn.example.com:3478?transport=tcp`.
   For TLS (port 5349, `turns:`) give coturn a certificate (`--cert`, `--pkey`).
3. Open the firewall: **3478 UDP+TCP** (and 5349 TCP for TLS) and the relay range **49160–49200 UDP**
   (one port per relayed stream; widen it for many concurrent calls).
4. Keep the `--denied-peer-ip` lines: they stop the relay from being used to reach your private networks.

Check: open `GET /api/v1/calls/ice` while signed in — it lists the servers and a fresh TURN credential.
In the browser, `chrome://webrtc-internals` shows whether a call uses `host`, `srflx` (STUN) or `relay` (TURN)
candidates.

## Several Kyber replicas

With `KYBER_DATABASE_URL` set, realtime events (chat messages, typing, calls, signaling, whiteboard changes)
cross replicas through PostgreSQL `LISTEN/NOTIFY` on the channel `kyber_realtime`; events larger than
NOTIFY's 8 kB (SDP offers, big whiteboard batches) go through the `realtime_spill` table, which keeps rows for
minutes. No sticky sessions are needed. A browser that reconnects to another replica reloads what it shows
instead of replaying missed events (event ids are per replica). Presence (`GET /api/v1/presence`) only knows
the streams of the replica that answers — a known limit.

The reverse proxy must not buffer `text/event-stream` responses (`/api/v1/stream`, `/api/v1/events`): Kyber
sends `X-Accel-Buffering: no` for nginx; keep read timeouts above the 25-second heartbeat.

## Files

Chat files and huddle recordings are kept under `KYBER_FILES_DIR` (one file per message id, two levels deep).
Back the directory up together with the database. Without it, files live in memory and vanish on restart (fine
for a demo). With several replicas, put the directory on shared storage — or wait for the S3 adapter.

## Recording transcripts (optional)

Set `KYBER_STT_URL` to an OpenAI-compatible speech-to-text endpoint (for example a self-hosted Whisper server's
`/v1/audio/transcriptions`) and each huddle recording gets a "📝 Transcript" in its thread. `KYBER_STT_MODEL`
defaults to `whisper-1`; `KYBER_STT_KEY` (a secret — set it in the environment, never in files) is sent as a
bearer token. Unset, recordings never leave the server. Transcripts are capped at 4000 characters.

## Metrics

`GET /metrics` adds:

| Metric | Meaning |
|---|---|
| `kyber_stream_connections` | realtime streams open on this replica (≈ open browser tabs) |
| `kyber_calls_active` | live huddles in all workspaces |

Upload bandwidth per participant grows with the call size in a mesh (n − 1 streams): plan about 1.5 Mbit/s up
per video peer; the 6-person cap keeps a call within ordinary home connections. A media server (SFU) for
bigger calls and recording is a later ADR.
