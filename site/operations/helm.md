# Helm chart

The chart lives in [`deploy/helm/kyber`](https://github.com/bakhod1r/kyber/tree/main/deploy/helm/kyber) and
deploys the `ghcr.io/bakhod1r/kyber` image (tag defaults to the chart's `appVersion`).

```bash
kubectl create secret generic kyber \
  --from-literal=KYBER_DATABASE_URL='postgres://kyber:…@db:5432/kyber'
helm install kyber ./deploy/helm/kyber \
  --set env.KYBER_PUBLIC_URL=https://kyber.example.com \
  --set ingress.enabled=true --set 'ingress.hosts[0].host=kyber.example.com'
```

## Values

| Value | Default | Notes |
|---|---|---|
| `replicaCount` | `2` | Migrations run at startup under an advisory lock, so replicas may start together |
| `existingSecret` | `kyber` | Secrets are referenced, never stored in values. Must hold `KYBER_DATABASE_URL`; may hold `KYBER_REDIS_URL`, `KYBER_S3_ACCESS_KEY`, `KYBER_S3_SECRET_KEY`, `KYBER_SMTP_PASSWORD`, `KYBER_TELEGRAM_BOT_TOKEN`, OAuth secrets |
| `env` | `KYBER_PUBLIC_URL`, `KYBER_COOKIE_SECURE: "true"` | Plain settings — any variable from the [configuration reference](../getting-started/configuration) |
| `attachments.persistence.*` | disabled, `ReadWriteMany`, `20Gi` | With more than one replica use S3 (`KYBER_S3_*`) or a ReadWriteMany volume |
| `mqtt.enabled` / `mqtt.port` | `false` / `1883` | Plain MQTT for integrations; sets `KYBER_MQTT_ADDR` and exposes the port on the Service |
| `service.type` / `service.port` | `ClusterIP` / `80` | |
| `ingress.*` | disabled | MQTT over WebSocket (`/api/v1/mqtt`) is long-lived — raise proxy timeouts |
| `resources` | requests `100m` / `128Mi`, limit `512Mi` | |
| `podSecurityContext`, `securityContext` | non-root `65532`, read-only root FS, no capabilities | |

Connect to PostgreSQL as a non-superuser role so row-level security applies.

Health probes: `/healthz` (liveness) and `/readyz` (database). Upgrades are normal rolling updates — see
[Backup & restore](./backup#upgrade).
