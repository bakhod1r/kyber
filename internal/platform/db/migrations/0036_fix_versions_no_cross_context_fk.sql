-- ADR-0005: bounded contexts refer to each other by id only. Issue Tracking's fix versions no longer
-- cascade from Agile's versions table: deleting a version publishes version.deleted and Issue
-- Tracking moves or drops the issues itself (so their history records the change).
ALTER TABLE issue_fix_versions DROP CONSTRAINT issue_fix_versions_version_id_fkey;
