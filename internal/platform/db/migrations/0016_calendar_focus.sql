-- Sprint 09 (KYB-S41/S42): meetings on the time table, and Pomodoro focus sessions (the work log).
CREATE TABLE meetings (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id),
    title        text NOT NULL CHECK (btrim(title) <> ''),
    starts_at    timestamptz NOT NULL,
    ends_at      timestamptz NOT NULL CHECK (ends_at > starts_at),
    organizer_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE
);
CREATE TABLE meeting_attendees (
    meeting_id uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (meeting_id, user_id)
);
CREATE INDEX meeting_attendees_user ON meeting_attendees (user_id);
CREATE INDEX meetings_time ON meetings (starts_at, ends_at);

CREATE TABLE focus_sessions (
    id           uuid PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id),
    issue_key    text NOT NULL,
    planned_s    integer NOT NULL CHECK (planned_s BETWEEN 900 AND 3600),
    started_at   timestamptz NOT NULL,
    state        text NOT NULL CHECK (state IN ('running', 'paused', 'completed', 'interrupted')),
    paused_at    timestamptz,
    paused_s     double precision NOT NULL DEFAULT 0,
    ended_at     timestamptz,
    reason       text NOT NULL DEFAULT ''
);
-- One current session per user, enforced by the database.
CREATE UNIQUE INDEX focus_sessions_one_active ON focus_sessions (user_id) WHERE state IN ('running', 'paused');
CREATE INDEX focus_sessions_issue ON focus_sessions (workspace_id, issue_key, started_at DESC);
CREATE INDEX focus_sessions_user ON focus_sessions (user_id, started_at DESC);
