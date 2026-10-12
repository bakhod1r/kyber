-- S47: an unfinished issue carried over when its sprint was completed (for the sprint report).
ALTER TABLE activity DROP CONSTRAINT activity_kind_check;
ALTER TABLE activity ADD CONSTRAINT activity_kind_check CHECK (kind IN ('created', 'status', 'sprint', 'estimate', 'carried'));
