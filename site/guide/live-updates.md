# Live updates (MQTT)

Kyber embeds an MQTT broker (MQTT 3.1.1 and 5), so a deployment needs no separate Mosquitto or EMQX.

## Transports

- **Browsers** connect with MQTT over WebSocket at `GET /api/v1/mqtt` (subprotocol `mqtt`), authenticated like
  any other request. The web app opens one connection per tab.
- **Integrations** can use plain MQTT when `KYBER_MQTT_ADDR` is set (e.g. `:1883`): username = workspace id,
  password = a [personal access token](./ai-assistants#option-2-personal-access-token). Terminate TLS in front
  of this port.

## Topics

| Topic | Payload | Content |
|---|---|---|
| `kyber/<workspace>/projects/<KEY>/events` | `{"project","event"}` | Hints only — refetch details through the API |
| `kyber/<workspace>/users/<user>/events` | `{"type","data"}` | Your own chat, call and whiteboard events |

Clients never publish. They may subscribe with `+` for the workspace and/or project (or user); every delivery is
re-checked against your workspace and project access, so a revoked membership stops hints at once. You cannot
subscribe to another user's personal topic.

Messages are QoS 0 and not retained: after a reconnect, refetch what you show.

```sh
mosquitto_sub -h kyber.example.com -p 1883 -u "$WORKSPACE_ID" -P "$KYBER_TOKEN" \
  -t "kyber/+/projects/KYB/events"
```

With several replicas, hints cross nodes through PostgreSQL `LISTEN/NOTIFY`; no sticky sessions are needed.
The reverse proxy must pass WebSocket upgrades on `/api/v1/mqtt` and keep idle timeouts above the MQTT
keepalive.

Design: [ADR-0013](https://github.com/bakhod1r/kyber/blob/main/docs/adr/0013-mqtt-live-updates.md).
