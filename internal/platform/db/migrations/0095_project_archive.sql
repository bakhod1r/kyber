-- B10: an archived project is read-only until an admin restores it.
ALTER TABLE projects ADD COLUMN archived boolean NOT NULL DEFAULT false;
