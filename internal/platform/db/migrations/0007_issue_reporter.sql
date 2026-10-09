-- Sprint 06 (KYB-S21): the user who created the issue. Older issues keep NULL:
-- their creator was never recorded and must not be guessed.
ALTER TABLE issues ADD COLUMN reporter_id uuid REFERENCES users (id) ON DELETE SET NULL;
