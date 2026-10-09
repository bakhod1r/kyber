-- QA-05-1: optimistic locking for sprints (concurrent starts of the same sprint
-- all succeeded and wrote duplicate sprint.started events).
ALTER TABLE sprints ADD COLUMN version integer NOT NULL DEFAULT 1;
