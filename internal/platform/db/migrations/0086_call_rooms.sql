-- S99 (ADR-0017): a call's media goes through the media server (LiveKit) instead of a browser mesh.
ALTER TABLE calls ADD COLUMN room boolean NOT NULL DEFAULT false;
