-- C6: the event envelope (actor, trace context, schema version) and dead letters. A message whose
-- handlers fail too often is parked (dead_at) so it stops blocking the queue; an operator retries it.
ALTER TABLE outbox
    ADD COLUMN actor_id       text NOT NULL DEFAULT '',
    ADD COLUMN trace_parent   text NOT NULL DEFAULT '',
    ADD COLUMN schema_version int  NOT NULL DEFAULT 1,
    ADD COLUMN attempts       int  NOT NULL DEFAULT 0,
    ADD COLUMN last_error     text NOT NULL DEFAULT '',
    ADD COLUMN dead_at        timestamptz;

DROP INDEX outbox_unpublished;
CREATE INDEX outbox_unpublished ON outbox (id) WHERE published_at IS NULL AND dead_at IS NULL;
CREATE INDEX outbox_dead ON outbox (dead_at) WHERE dead_at IS NOT NULL;
