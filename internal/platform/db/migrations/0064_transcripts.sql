-- Sprint 12 (S77): transcripts of huddle recordings, written in the recording's thread.
ALTER TABLE messages DROP CONSTRAINT messages_kind_check;
ALTER TABLE messages ADD CONSTRAINT messages_kind_check
    CHECK (kind IN ('', 'huddle_started', 'huddle_ended', 'huddle_recording', 'project_event', 'transcript'));
