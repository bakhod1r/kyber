# Sprint 16 — Large huddles through a media server

Source: the last open item of the plan (2026-10-14): huddles stop at six people because media is a browser mesh.
ADR-0017 compared Pion, LiveKit, mediasoup and Janus; the product owner chose **LiveKit, optional** ("b"). Planned
first, then built DDD + TDD (domain and application tests, the repository contract on memory and PostgreSQL,
acceptance tests, and a Playwright e2e against a real LiveKit server).

- ✅ **S99 Room calls in the domain** — a call is a mesh (≤ 6) or a room on the media server (≤ 50); a room has no
  browser-to-browser signaling; only participants enter it. AC: limits per mode; persisted.
  **Done:** `Call.Room`, `StartRoom`, `Limit`, `EnterRoom`; migration 0086; repository contract.
- ✅ **S100 Room tickets** — `Rooms` port; with a media server new calls are rooms; `POST /calls/{id}/room` returns a
  short-lived LiveKit token for that room only (API 0.70.0). AC: outsiders 404, members not in the call 409,
  a mesh call 409; the secret never leaves the server.
  **Done:** `infrastructure/livekit` (HS256 JWT, 15 min, no admin grant), `KYBER_LIVEKIT_URL/_KEY/_SECRET`, CSP
  `connect-src` for that origin only when set.
- ✅ **S101 The web app in rooms** — same huddle UI; the engine publishes microphone, camera and screen once to the
  room and shows what LiveKit forwards; `livekit-client` loads only for room calls; "full" follows the mode.
  **Done:** `RoomConnection` port in the call domain, LiveKit adapter in infrastructure. Fixed on the way: updates
  that arrived while connecting to the room were overwritten by the join's older answer.
- ✅ **S102 Operations** — `docker compose --profile rooms`, ops guide, CI e2e with `livekit-server --dev`:
  seven people, camera and screen through LiveKit, no mesh signals.

- ✅ **S103 Remove people** (product request, 2026-10-14) — the host removes someone from the huddle; whoever
  leaves or is removed is dropped from the LiveKit room by the server. AC: only the host; the removed person is told;
  a closed tab cannot keep listening.
  **Done:** `Call.Remove`, `ParticipantLeft` event → `RemoveParticipant`; `DELETE /calls/{id}/participants/{user}`
  (403 `CALL_NOT_HOST`, API 0.71.0); ✕ on tiles for the host; "You were removed from the huddle".
- ✅ **S104 Server-side recording** (product request) — LiveKit Egress records room calls; the MP4 lands in the
  conversation like a browser recording (transcript and summary follow). AC: works without the recorder's browser
  staying connected; without Egress, unchanged.
  **Done:** `Recorder` port and `RecordOnServer`; `RecordingStopped` event → stop, wait for the file, post via the
  Chat ACL, delete it; `recording.server` in the API (migration 0087); `KYBER_LIVEKIT_RECORDINGS_DIR`/`_EGRESS_DIR`/
  `_API_URL`; compose `egress` service. Verified against a real Egress (`make e2e-egress`, in CI: Redis, LiveKit and
  `livekit/egress` in Docker — the MP4 lands in the channel, nothing is left in the directory). That run found
  Egress's `<id>.json` manifest left behind; it is deleted with the file now.

Not planned (product owner, 2026-10-14): a Helm chart for LiveKit and an embedded Pion SFU.
