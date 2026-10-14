-- Sprint 14 (S93): a stored layer order for whiteboard elements (bring to front / send to back).
ALTER TABLE whiteboard_elements ADD COLUMN z double precision NOT NULL DEFAULT 0;
