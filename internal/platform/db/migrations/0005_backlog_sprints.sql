-- Sprint 05: Scrum sprints (KYB-S19) and backlog ranking (KYB-S18).
CREATE TABLE sprints (
    id           uuid PRIMARY KEY,
    project_key  text NOT NULL REFERENCES projects (key),
    name         text NOT NULL CHECK (btrim(name) <> ''),
    goal         text NOT NULL DEFAULT '',
    state        text NOT NULL CHECK (state IN ('planned', 'active', 'closed')),
    started_at   timestamptz,
    completed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
-- At most one active sprint per project, enforced by the database as well as the domain.
CREATE UNIQUE INDEX sprints_one_active ON sprints (project_key) WHERE state = 'active';
CREATE INDEX sprints_project ON sprints (project_key, created_at);

-- Ranks are fractional indexes compared byte-wise, hence COLLATE "C".
ALTER TABLE issues
    ADD COLUMN rank      text COLLATE "C",
    ADD COLUMN sprint_id uuid REFERENCES sprints (id) ON DELETE SET NULL;

-- Backfill existing issues in number order with valid fractional-index integers:
-- head 'c' = 3 digits, 'd' = 4, ... (heads ascend, so order holds across lengths).
UPDATE issues SET rank = CASE
    WHEN number < 1000      THEN 'c' || lpad(number::text, 3, '0')
    WHEN number < 10000     THEN 'd' || lpad(number::text, 4, '0')
    WHEN number < 100000    THEN 'e' || lpad(number::text, 5, '0')
    ELSE                         'f' || lpad(number::text, 6, '0')
END;
ALTER TABLE issues ALTER COLUMN rank SET NOT NULL;
CREATE INDEX issues_rank ON issues (project_key, rank, number);
CREATE INDEX issues_sprint ON issues (sprint_id) WHERE sprint_id IS NOT NULL;
