# ADR-0013: Live updates over MQTT

Status: accepted (2026-10-14). Supersedes ADR-0008 (Server-Sent Events).

## Context

Live hints went to browsers over SSE. The team wants one protocol for every live consumer:
browsers, scripts, dashboards and devices. MQTT is that protocol; browsers speak it over
WebSocket.

## Decision

- **Embedded broker.** Kyber runs its own MQTT broker (mochi-mqtt, MQTT 3.1.1 and 5) in-process,
  so a deployment needs no Mosquitto/EMQX. The server publishes through the broker's inline client.
- **Transports.** Browsers: MQTT over WebSocket at `GET /api/v1/mqtt` (subprotocol `mqtt`), behind
  the normal authentication and workspace middleware; cookie-authenticated upgrades are accepted
  from the same origin only. Other clients: optional plain MQTT on `KYBER_MQTT_ADDR` (e.g. `:1883`)
  with username = workspace id and password = a personal access token (ADR-0012); terminate TLS in
  front of it.
- **Topics.** `kyber/<workspace>/projects/<KEY>/events`, payload `{"project","event"}` — hints,
  never content (ADR-0008's rule stands); clients refetch through the API.
- **Authorization.** Clients never publish. They may subscribe to project topics, with `+` for the
  workspace and/or the project. Every delivery is a concrete topic and is re-checked: it must be the
  client's workspace and the user must be able to browse the project, so a revoked membership stops
  hints at once. Client ids are prefixed with the user id, so nobody can take over another user's
  session.
- **Replicas.** Unchanged: the relay's hints go through `NOTIFY kyber_live`, and every replica's
  listener publishes into its own broker.

## Consequences

- One protocol for browsers and integrations; QoS 0, no retained messages, so a client that was
  offline refetches on reconnect as before.
- The web app loads mqtt.js lazily for signed-in pages.
- Presence (who is viewing an issue, F4) uses the same broker: the issue panel sends
  `POST /api/v1/issues/{key}/presence` every 20 s, the replica keeps (user, issue) in memory for 45 s,
  and a new viewer triggers a `presence.changed` hint on the issue's project (project only, never the
  key, per the rule above). Presence is per replica: without sticky sessions a viewer is listed only
  by the replica that received their heartbeats; clients also poll every 20 s to notice people leaving.

## Amendment (2026-10-14, S88): personal topics for chat, calls and whiteboards

- **Topic.** `kyber/<workspace>/users/<user>/events`, payload `{"type", "data"}`: the same events the SSE
  stream (`GET /api/v1/stream`, ADR-0009) carries. Unlike project hints these carry content (a chat message,
  call signaling), so they go only to the people the event is addressed to.
- **Authorization.** A client may subscribe with `+` for the workspace and/or the user; every delivery is a
  concrete topic and must name the client's own workspace and user. Subscribing to another user's topic
  by name is refused.
- **Publishing.** The realtime hub hands every event to the broker as well (`Hub.Forward`); across replicas
  the existing `NOTIFY kyber_realtime` feed reaches every node's hub, and each publishes into its own broker.
- **Web app.** One MQTT connection per tab carries chat, calls, whiteboards and board hints; `onConnect` reloads after a reconnect
  (QoS 0, no replay). `GET /api/v1/stream` stays for existing scripts; the web app no longer opens it.
- **Presence** counts MQTT clients as well as SSE streams (on the replica that answers).

## Amendment (2026-10-15, S105): presence across replicas

- **Problem.** `GET /api/v1/presence` only knew the MQTT clients and SSE streams of the replica that answered.
- **Options.** (a) Share connect/disconnect through `NOTIFY` — cheap, but a replica that starts late has no
  history, and a crashed replica never sends its disconnects, leaving users "online" forever. (b) A table of
  seats with heartbeats and expiry. **Chosen: (b).**
- **Design** (`internal/platform/presence`). Each replica beats every 15 s, and at once when a client connects
  or leaves, replacing its own rows in `presence (replica, workspace_id, user_id, seen_at)` with the users
  connected to it (MQTT clients and SSE streams). A user is online while a seat was seen in the last 45 s
  (three beats, so one lost beat does not flicker); stale rows are expired by every replica. A clean
  shutdown clears the replica's rows; a crashed replica's users go offline within 45 s. The answering
  replica also counts its own connections immediately, before its next beat. Dev mode uses the same roster
  over an in-memory store.
- **Cost.** One small transaction per replica per beat; presence queries use the
  `(workspace_id, user_id, seen_at)` index. The issue panel's "also viewing" (F4) is unchanged.
