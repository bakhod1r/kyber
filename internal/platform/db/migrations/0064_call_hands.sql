-- Sprint 12 (S75): a raised hand stays on the participant until lowered.
ALTER TABLE call_participants ADD COLUMN hand boolean NOT NULL DEFAULT false;
