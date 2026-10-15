-- S107c (ADR-0018 §5): how a session's user signed in ("password", "google", "sso:<connection id>", …),
-- so a workspace that requires SSO can tell. Sessions from before this migration have none and count as
-- not signed in through SSO.
ALTER TABLE sessions ADD COLUMN auth_method text NOT NULL DEFAULT '';

-- A workspace's own connection can be required for its members (owners keep other ways in).
ALTER TABLE sso_connections ADD COLUMN enforced boolean NOT NULL DEFAULT false;
CREATE INDEX sso_connections_required ON sso_connections (workspace_id) WHERE enforced AND enabled;
