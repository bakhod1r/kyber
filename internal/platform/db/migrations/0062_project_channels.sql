-- Sprint 12 (S73): a channel can carry a project's news. The project belongs to the Project context: by key only.
ALTER TABLE conversations ADD COLUMN project_key text NOT NULL DEFAULT '';
CREATE INDEX conversations_project ON conversations (workspace_id, project_key) WHERE project_key <> '';

ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check
    CHECK (kind IN ('', 'huddle_started', 'huddle_ended', 'huddle_recording', 'project_event'));
