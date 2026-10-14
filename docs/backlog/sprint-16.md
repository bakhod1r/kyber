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

Follow-ups (not planned): remove a participant from the LiveKit room on leave/kick (server API), LiveKit Egress
recording, an embedded Pion SFU adapter (ADR-0017 option A).
