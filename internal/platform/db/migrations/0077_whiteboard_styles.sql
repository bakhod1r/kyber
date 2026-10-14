-- Sprint 14 (S91): stroke weight, fill and dashes of whiteboard elements; existing shapes keep the defaults.
ALTER TABLE whiteboard_elements
    ADD COLUMN stroke smallint NOT NULL DEFAULT 0 CHECK (stroke BETWEEN 0 AND 3),
    ADD COLUMN fill   boolean  NOT NULL DEFAULT false,
    ADD COLUMN dashed boolean  NOT NULL DEFAULT false;
