-- S105: who is online on any replica. Each replica replaces its own seats on every beat; a seat
-- counts while it is fresh, so a crashed replica's users age out (ADR-0013 amendment).
-- Written by the replicas without a workspace scope, like the outbox: no row-level security.
CREATE TABLE presence (
    replica      text        NOT NULL,
    workspace_id uuid        NOT NULL,
    user_id      uuid        NOT NULL,
    seen_at      timestamptz NOT NULL,
    PRIMARY KEY (replica, workspace_id, user_id)
);
CREATE INDEX presence_lookup ON presence (workspace_id, user_id, seen_at);
