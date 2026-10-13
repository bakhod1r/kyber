# ADR-0011: Attachments

Status: accepted (2026-10-13)

## Context

Issues need files: logs, screenshots, specs. Files are untrusted input served back from Kyber's
own origin, so a stored HTML or SVG file must never run as a page there.

## Decision

- **Context.** `internal/attachment` (four layers). Records (name, type, size, SHA-256, uploader)
  live in PostgreSQL (`attachments`, RLS per ADR-0010); bytes go through the `Blobs` port. The
  issue is referenced by key only.
- **Storage.** `blobfs` writes `<KYBER_ATTACHMENTS_DIR>/<workspace>/<id>` via a temporary file and
  rename; ids and workspaces are UUIDs, so paths cannot escape the directory. An in-memory store
  serves dev mode and tests. S3/MinIO is a later adapter behind the same port.
- **Limits.** At most 25 MiB per file, enforced while streaming (the request body is capped and the
  blob store reads at most the limit + 1 byte). Names are reduced to a safe base name; an
  unparsable content type becomes `application/octet-stream`.
- **Permissions** follow the issue through an ACL to Issue Tracking: browse to list and download,
  edit to upload; the uploader or a project administrator may remove a file. Files of hidden
  issues answer 404.
- **Safe downloads.** Only PNG, JPEG, GIF, WebP and PDF are served `inline` (and `?download`
  forces a download); everything else is `Content-Disposition: attachment`. Every response has
  `X-Content-Type-Options: nosniff` and `Content-Security-Policy: default-src 'none'; sandbox`.
- **CSRF.** Multipart cannot be JSON, so cookie-authenticated non-JSON writes must carry the
  `X-Kyber-Request` header; browsers cannot send it cross-site without a CORS preflight.

## Consequences

- The attachments directory must be backed up with the database; deleting a record leaves at most
  an orphan file (removed best effort).
- No virus scanning or image thumbnails yet; the UI shows images at small size from the original.
