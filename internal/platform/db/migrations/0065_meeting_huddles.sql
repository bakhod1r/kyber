-- Sprint 12 (S76): a meeting can happen in a chat channel's huddle (Chat context: by id only).
ALTER TABLE meetings ADD COLUMN conversation_id text NOT NULL DEFAULT '';
