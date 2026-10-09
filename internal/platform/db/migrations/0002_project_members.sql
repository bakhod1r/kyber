-- Sprint 03: project membership and roles (KYB-S11).
CREATE TABLE project_members (
    project_id uuid NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('admin', 'member', 'viewer')),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX project_members_user ON project_members (user_id);
