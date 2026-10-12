-- Audit log: every change made through the API (who, what, which project, when, result). Append-only.
CREATE TABLE audit_log (
    id           bigserial PRIMARY KEY,
    workspace_id uuid NOT NULL,
    at           timestamptz NOT NULL,
    actor_id     text NOT NULL,
    action       text NOT NULL,
    target       text NOT NULL DEFAULT '',
    project_key  text NOT NULL DEFAULT '',
    status       int NOT NULL
);
CREATE INDEX audit_log_project ON audit_log (workspace_id, project_key, at DESC, id DESC);
