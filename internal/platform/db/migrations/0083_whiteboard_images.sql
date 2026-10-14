-- Sprint 15 (S96): pictures on whiteboards (the bytes live in the blob store, keyed by board and image id).
ALTER TABLE whiteboard_elements DROP CONSTRAINT whiteboard_elements_kind_check;
ALTER TABLE whiteboard_elements ADD CONSTRAINT whiteboard_elements_kind_check
    CHECK (kind IN ('rect', 'ellipse', 'arrow', 'line', 'freehand', 'text', 'sticky', 'image'));
