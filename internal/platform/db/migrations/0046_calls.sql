-- Sprint 11 (S55, ADR-0006): huddles — live calls in a chat conversation. Users, workspaces and conversations
-- belong to other contexts: referenced by id only.
CREATE TABLE calls (
    id              uuid PRIMARY KEY,
    workspace_id    uuid NOT NULL,
    conversation_id uuid NOT NULL,
    started_by      uuid NOT NULL,
    started_at      timestamptz NOT NULL,
    ended_at        timestamptz,
    version         integer NOT NULL DEFAULT 0
);
-- One live call per conversation, enforced by the database.
CREATE UNIQUE INDEX calls_one_live ON calls (conversation_id) WHERE ended_at IS NULL;

CREATE TABLE call_participants (
    call_id   uuid NOT NULL REFERENCES calls (id) ON DELETE CASCADE,
    user_id   uuid NOT NULL,
    joined_at timestamptz NOT NULL,
    audio     boolean NOT NULL,
    video     boolean NOT NULL,
    screen    boolean NOT NULL,
    PRIMARY KEY (call_id, user_id)
);
