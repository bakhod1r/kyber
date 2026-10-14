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
- Presence (who is viewing an issue) can use the same broker later.

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
