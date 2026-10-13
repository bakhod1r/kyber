-- E2: team events. A meeting is for chosen people, a project's members or the workspace's; attendees
-- answer (RSVP) and are reminded once before the start. The project is referenced by key only.
ALTER TABLE meetings
    ADD COLUMN scope       text NOT NULL DEFAULT 'people' CHECK (scope IN ('people', 'project', 'workspace')),
    ADD COLUMN project_key text NOT NULL DEFAULT '',
    ADD COLUMN reminded_at timestamptz;
CREATE INDEX meetings_due ON meetings (starts_at) WHERE reminded_at IS NULL;

ALTER TABLE meeting_attendees
    ADD COLUMN response text NOT NULL DEFAULT '' CHECK (response IN ('', 'going', 'maybe', 'declined'));

ALTER TABLE notifications DROP CONSTRAINT notifications_kind_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_kind_check
    CHECK (kind IN ('assigned', 'commented', 'mentioned', 'transitioned', 'announcement', 'meeting'));
