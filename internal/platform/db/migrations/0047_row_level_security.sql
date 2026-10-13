-- Row-level security (ADR-0010): a connection scoped to a workspace (kyber.workspace, set by the
-- pool for every request) sees and writes only that workspace's rows. Unscoped connections
-- (migrations, relay, background jobs) keep full access. FORCE applies the policy to the table
-- owner too; superusers and BYPASSRLS roles are never restricted, so run Kyber as a plain role.
DO $$
DECLARE t text;
BEGIN
    FOR t IN
        SELECT c.table_name FROM information_schema.columns c
        JOIN information_schema.tables b ON b.table_schema = c.table_schema AND b.table_name = c.table_name
        WHERE c.table_schema = current_schema() AND c.column_name = 'workspace_id' AND b.table_type = 'BASE TABLE'
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
