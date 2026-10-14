# Running chat and huddles in production

Chat, huddles (audio, video, screen share), whiteboards and "present to all" need nothing beyond Kyber and
PostgreSQL — except that video calls between people on different networks need a STUN/TURN server.
Design: [ADR-0009](../adr/0009-chat-and-calls.md).

## How media flows

Kyber never touches audio or video. Browsers connect to each other (WebRTC, a full mesh of at most 6 people);
Kyber only relays the signaling (who wants to talk to whom) over MQTT (each user's own topic, ADR-0013). With a
media server configured (below), calls go through it instead and take up to 50 people.

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

## Large huddles: a media server (LiveKit)

In a mesh every browser sends its video to every other one, so a call stops at 6 people. With
[LiveKit](https://livekit.io) (open source, Apache-2.0) configured, every **new** huddle is a *room* on it:
each browser uploads once (simulcast) and LiveKit forwards to the others — up to 50 people (ADR-0017). Calls
already live keep their mode. Kyber still decides who may join: it signs a LiveKit token for one call's room,
valid for 15 minutes (LiveKit refreshes it for connected browsers), and only for people in the call.

```sh
docker compose --profile rooms up -d   # livekit-server, host networking: TCP 7880 (signaling), 7881, UDP 7882
```

| Setting | Value |
|---|---|
| `KYBER_LIVEKIT_URL` | where **browsers** reach LiveKit: `wss://sfu.example.com` behind TLS (`ws://` only on a LAN) |
| `KYBER_LIVEKIT_KEY` | an API key from LiveKit's `keys:` (or `LIVEKIT_KEYS="key: secret"`) |
| `KYBER_LIVEKIT_SECRET` | its secret — at least 32 random characters; environment only, never in files or logs |

Kyber refuses to start when the URL is set but is not `ws://`/`wss://` or the key or secret is missing. The
web app's Content-Security-Policy allows that origin (and a `blob:` worker LiveKit's client uses) only then.

- Put LiveKit behind TLS (a reverse proxy for 7880, or LiveKit's own `turn`/`tls` settings); open UDP 7882 and
  TCP 7881 to the internet. Behind NAT set LiveKit's `rtc.use_external_ip: true` or `node_ip`.
- Strict networks: LiveKit has an embedded TURN server (`turn.enabled`), or keep coturn.
- Sizing: LiveKit forwards, it does not transcode — roughly 1.5 Mbit/s per viewer per shown video; a small VM
  handles a few 20-person calls.
- The host (who started the call) can remove someone (✕ on their tile, `DELETE /calls/{id}/participants/{user}`);
  Kyber then removes them from the LiveKit room too (`RemoveParticipant`, through the outbox, retried). The same
  happens to anyone who leaves, so a closed tab cannot keep listening. `KYBER_LIVEKIT_API_URL` sets where Kyber
  calls LiveKit's API when that differs from the browsers' address (e.g. `http://livekit:7880` inside a cluster).
- Several LiveKit nodes need Redis (LiveKit's own `redis:` setting); Kyber replicas need nothing extra.

### Server-side recording (LiveKit Egress)

Without Egress a room call is recorded by the browser of whoever presses **Record**. With
[Egress](https://docs.livekit.io/home/egress/overview/) (`docker compose --profile rooms up` starts it with Redis)
the media server records instead: one MP4 with everyone in a grid, independent of anyone's connection.

| Setting | Value |
|---|---|
| `KYBER_LIVEKIT_RECORDINGS_DIR` | the directory Egress writes into, as Kyber sees it (compose: the `recordings` volume at `/data/recordings`) |
| `KYBER_LIVEKIT_EGRESS_DIR` | the same directory as Egress sees it (default `/out`) |

When the recording stops (or its recorder leaves), Kyber stops the egress, waits up to 30 seconds for the file,
posts it to the conversation as the recorder's "huddle recording" (transcript and summary follow when configured)
and deletes it from the directory. A file still being written is retried by the outbox. Kyber only opens plain
file names inside that directory, as LiveKit reports them. Egress needs about one CPU core per recording
(it runs Chrome); give it `SYS_ADMIN` or see LiveKit's docs for running without.
- Turning LiveKit off: room calls still live cannot be entered (409); new calls are a mesh again.

## Several Kyber replicas

With `KYBER_DATABASE_URL` set, realtime events (chat messages, typing, calls, signaling, whiteboard changes)
cross replicas through PostgreSQL `LISTEN/NOTIFY` on the channel `kyber_realtime`; events larger than
NOTIFY's 8 kB (SDP offers, big whiteboard batches) go through the `realtime_spill` table, which keeps rows for
minutes. No sticky sessions are needed. A browser that reconnects to another replica reloads what it shows
instead of replaying missed events (event ids are per replica). Presence (`GET /api/v1/presence`) only knows
the streams of the replica that answers — a known limit.

The reverse proxy must pass WebSocket upgrades on `/api/v1/mqtt` (the web app's only realtime connection,
S88) and keep idle timeouts above MQTT's keepalive. It must not buffer `text/event-stream` responses
(`/api/v1/stream`, kept for scripts): Kyber
sends `X-Accel-Buffering: no` for nginx; keep read timeouts above the 25-second heartbeat.

## Files

Chat files and huddle recordings are kept under `KYBER_FILES_DIR` (one file per message id, two levels deep).
Back the directory up together with the database. Without it, files live in memory and vanish on restart (fine
for a demo). With several replicas, use S3/MinIO: when `KYBER_S3_BUCKET` (and the other `KYBER_S3_*` settings) are
set, chat files and recordings go to the same bucket as issue attachments, under the `chat/` prefix (S82).

## Recording transcripts (optional)

Set `KYBER_STT_URL` to an OpenAI-compatible speech-to-text endpoint (for example a self-hosted Whisper server's
`/v1/audio/transcriptions`) and each huddle recording gets a "📝 Transcript" in its thread. `KYBER_STT_MODEL`
defaults to `whisper-1`; `KYBER_STT_KEY` (a secret — set it in the environment, never in files) is sent as a
bearer token. Unset, recordings never leave the server. Transcripts are capped at 4000 characters.

With transcripts on, set `KYBER_LLM_URL` to an OpenAI-compatible chat completions endpoint (for example a
self-hosted Ollama's `/v1/chat/completions`) and each transcript also gets a "🧠 Summary" (decisions, action items,
open questions) in the same thread (S87). `KYBER_LLM_MODEL` defaults to `gpt-4o-mini`; `KYBER_LLM_KEY` is a secret
like `KYBER_STT_KEY`. At most 16000 characters of a transcript are sent. Unset, transcripts never leave the server.

## Metrics

`GET /metrics` adds:

| Metric | Meaning |
|---|---|
| `kyber_stream_connections` | realtime streams open on this replica (≈ open browser tabs) |
| `kyber_calls_active` | live huddles in all workspaces |

Upload bandwidth per participant grows with the call size in a mesh (n − 1 streams): plan about 1.5 Mbit/s up
per video peer; the 6-person cap keeps a call within ordinary home connections. For bigger calls configure a
media server (above): each participant then uploads once.
