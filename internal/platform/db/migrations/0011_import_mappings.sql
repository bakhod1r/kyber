-- Sprint 08 (KYB-S29): Importer context. One row per external issue makes imports idempotent.
CREATE TABLE import_mappings (
    project_key  text NOT NULL,
    external_key text NOT NULL,
    issue_key    text NOT NULL,
    imported_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_key, external_key)
);
