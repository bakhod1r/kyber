-- S104 (ADR-0017): the media server's recording of a room call (LiveKit egress id), while it records.
ALTER TABLE calls ADD COLUMN recording_egress text;
