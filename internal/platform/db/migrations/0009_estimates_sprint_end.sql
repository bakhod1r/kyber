-- Sprint 07 (KYB-S25): story points (tenths, so sums are exact) and sprint end dates.
ALTER TABLE issues ADD COLUMN estimate_tenths integer CHECK (estimate_tenths BETWEEN 0 AND 9990);
ALTER TABLE sprints ADD COLUMN ends_at timestamptz;
-- Sprints already running get Jira's default length.
UPDATE sprints SET ends_at = started_at + interval '14 days' WHERE started_at IS NOT NULL AND ends_at IS NULL;
