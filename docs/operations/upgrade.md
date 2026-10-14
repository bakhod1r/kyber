# Upgrading Kyber

Kyber follows semantic versioning. A patch or minor release upgrades in place; read the release notes
of every version you skip before a major upgrade.

## What happens on upgrade

- **Schema migrations run automatically at start-up.** The SQL files embedded in the binary
  (`internal/platform/db/migrations`) are applied in order, in one transaction, under a PostgreSQL
  advisory lock — so when several replicas start together, one migrates and the others wait, then
  find nothing left to do. Applied migrations are recorded in `schema_migrations`.
- **Migrations are forward-only.** There are no down migrations: to go back, restore the backup
  taken before the upgrade (see [backup.md](backup.md)) and run the previous version.
- **A failed migration rolls back completely** (one transaction) and the process exits; the database
  stays on the old schema and the previous version keeps working.
- Migrations aim to stay compatible with the previous release's code (columns are added with
  defaults, constraints widened), so a rolling upgrade can run old and new replicas side by side for
  the minutes it takes.

## Checklist

1. Read the release notes (breaking changes, new required settings).
2. **Back up** the database and the file store ([backup.md](backup.md)). Run the dump as the admin role.
3. Upgrade **one** replica first (or run the new binary once against the database) and watch its log
   for `migration … ` errors; `GET /readyz` must answer `ok`.
4. Roll the remaining replicas.
5. Check `kyber outbox dead-letters` for events that failed during the switch, and retry them with
   `kyber outbox retry <id>` once the cause is fixed (ADR-0007).

## Docker Compose

```sh
docker compose pull            # or: docker compose build, when building from source
docker compose up -d kyber     # migrates on start
docker compose logs -f kyber   # wait for "listening"
```

## Helm

```sh
helm upgrade kyber deploy/helm/kyber --reuse-values --set image.tag=<new version>
kubectl rollout status deployment/kyber
```

The Deployment does a rolling update; the advisory lock makes concurrent start-ups safe.
To roll back the code without the schema: `helm rollback kyber` works only while the newer
migrations are backward compatible (the usual case). After an incompatible migration, restore the
pre-upgrade backup instead.

## Binary

Stop the service, replace the binary, start it. With systemd:

```sh
sudo systemctl stop kyber && sudo install kyber /usr/local/bin/kyber && sudo systemctl start kyber
```
