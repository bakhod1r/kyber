-- Row-level security (ADR-0010) for the chat, call and whiteboard tables, created after 0047: every
-- table with a workspace_id column that has no policy yet gets the same tenant_isolation policy.
DO $$
DECLARE t text;
BEGIN
    FOR t IN
        SELECT c.table_name FROM information_schema.columns c
        JOIN information_schema.tables b ON b.table_schema = c.table_schema AND b.table_name = c.table_name
        JOIN pg_class k ON k.relname = c.table_name AND k.relnamespace = current_schema()::regnamespace
        WHERE c.table_schema = current_schema() AND c.column_name = 'workspace_id' AND b.table_type = 'BASE TABLE'
          AND NOT k.relrowsecurity
    LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
        EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        EXECUTE format($p$CREATE POLICY tenant_isolation ON %I
            USING (nullif(current_setting('kyber.workspace', true), '') IS NULL
                   OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)
            WITH CHECK (nullif(current_setting('kyber.workspace', true), '') IS NULL
                   OR workspace_id = nullif(current_setting('kyber.workspace', true), '')::uuid)$p$, t);
    END LOOP;
END $$;
