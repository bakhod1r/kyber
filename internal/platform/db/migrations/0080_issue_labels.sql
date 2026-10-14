-- Labels: free-form tags on an issue (Jira labels), filtered with JQL `labels = x`.
ALTER TABLE issues ADD COLUMN labels text[] NOT NULL DEFAULT '{}';
CREATE INDEX issues_labels_idx ON issues USING gin (labels);
