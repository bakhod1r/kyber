-- Worklog (Jira's time tracking): estimates and logged time on the issue, the worklog list as a projection.
ALTER TABLE issues
    ADD COLUMN original_estimate_s  bigint NOT NULL DEFAULT 0,
    ADD COLUMN remaining_estimate_s bigint NOT NULL DEFAULT 0,
    ADD COLUMN time_spent_s         bigint NOT NULL DEFAULT 0;

CREATE TABLE issue_worklogs (
    id         uuid PRIMARY KEY,
    issue_id   uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    author_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    started    timestamptz NOT NULL,
    seconds    bigint NOT NULL CHECK (seconds > 0),
    comment    text NOT NULL DEFAULT ''
);
CREATE INDEX issue_worklogs_issue ON issue_worklogs (issue_id, started);
