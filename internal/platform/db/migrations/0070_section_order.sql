-- Sprint 13 (S85): the order of each person's own sidebar sections.
CREATE TABLE chat_section_orders (
    workspace_id uuid NOT NULL,
    user_id      uuid NOT NULL,
    sections     text[] NOT NULL,
    PRIMARY KEY (workspace_id, user_id)
);

ALTER TABLE chat_section_orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_section_orders FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON chat_section_orders
    USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
    WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
           OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid);
