-- KYB-S45: issue hierarchy (epic → story/task/bug → sub-task). Removing a parent leaves its children in place.
ALTER TABLE issues ADD COLUMN parent_id uuid REFERENCES issues (id) ON DELETE SET NULL;
CREATE INDEX issues_parent ON issues (parent_id) WHERE parent_id IS NOT NULL;
