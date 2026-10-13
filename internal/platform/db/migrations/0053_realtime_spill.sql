-- Sprint 11 (S58, ADR-0009): realtime events larger than NOTIFY's 8 kB payload (SDP offers, whiteboard
-- batches) cross replicas through this table; NOTIFY carries only the row id. Rows live for minutes.
CREATE TABLE realtime_spill (
    id         bigserial PRIMARY KEY,
    payload    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX realtime_spill_created ON realtime_spill (created_at);
