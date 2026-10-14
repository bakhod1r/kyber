-- Sprint 11 (S52b, ADR-0006): mentions in chat land in the same inbox. The conversation belongs to the Chat
-- context: referenced by id only (no cross-context FK).
ALTER TABLE notifications ADD COLUMN conversation_id text NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN conversation_name text NOT NULL DEFAULT '';
