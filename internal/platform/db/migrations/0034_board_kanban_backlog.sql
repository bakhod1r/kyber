-- S48: Jira's Kanban backlog — a Kanban board can list its first column's issues apart.
ALTER TABLE boards ADD COLUMN kanban_backlog boolean NOT NULL DEFAULT false;
