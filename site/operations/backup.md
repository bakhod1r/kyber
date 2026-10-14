# Backup & restore

Adapted from [`docs/operations/backup.md`](https://github.com/bakhod1r/kyber/blob/main/docs/operations/backup.md).

Kyber keeps state in two places: **PostgreSQL** (everything except file contents) and the **file store**
(`KYBER_ATTACHMENTS_DIR` / `KYBER_FILES_DIR`, or the S3 bucket). Back up both, at the same time. In-memory dev
mode (no `KYBER_DATABASE_URL`) keeps nothing.

## Backup

```sh
# 1. Database: a consistent logical dump (online; no downtime).
pg_dump --format=custom --no-owner --file=kyber-$(date +%F).dump "$ADMIN_DATABASE_URL"

# 2a. Attachments on disk: copy after the dump (files are written via rename, so a copy never
#     sees a partial file; a file uploaded after the dump is merely unused after restore).
rsync -a "$KYBER_ATTACHMENTS_DIR"/ backup-host:/backups/kyber-attachments/

# 2b. Attachments in S3/MinIO: enable bucket versioning, or mirror the bucket:
mc mirror --overwrite kyber/kyber-attachments backup/kyber-attachments
```

Run the dump as the admin (superuser) role: the application role is subject to row-level security, and a dump
through it could miss rows. Keep at least daily dumps for 30 days and test a restore regularly.

## Restore

```sh
# Stop Kyber (or scale to 0), then restore into an empty database owned by the application role.
createdb --owner=kyber kyber_restored
pg_restore --no-owner --role=kyber --dbname="$ADMIN_DATABASE_URL_FOR_kyber_restored" kyber-2026-10-14.dump
rsync -a backup-host:/backups/kyber-attachments/ "$KYBER_ATTACHMENTS_DIR"/
# Point KYBER_DATABASE_URL at the restored database and start Kyber.
```

Outbox events that were pending at backup time are delivered again after a restore; handlers are idempotent,
but notification emails may repeat.

## Upgrade

1. Read the release notes; take a backup (above).
2. Deploy the new version. Migrations run automatically at startup, forward-only, under an advisory lock, so
   several replicas can start at once (Helm: a normal rolling update).
3. Watch `/readyz` and the logs. Rolling back the binary is safe only if the release notes say the new
   migrations are backward compatible; otherwise restore the backup.

## Dead letters

`kyber outbox dead-letters` lists events that failed 10 times; `kyber outbox retry <id>` queues one again.
