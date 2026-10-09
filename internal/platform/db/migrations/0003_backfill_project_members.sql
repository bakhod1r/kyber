-- QA-03-1: projects created before 0002 have no members and would become invisible.
-- Before 0.3 every signed-in user could access every project, so preserve that access
-- by making all existing users admins of member-less projects. Idempotent.
INSERT INTO project_members (project_id, user_id, role)
SELECT p.id, u.id, 'admin'
FROM projects p CROSS JOIN users u
WHERE NOT EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = p.id)
ON CONFLICT DO NOTHING;
