-- Search: saved JQL filters (private to their owner, or shared with the workspace).
CREATE TABLE saved_filters (
    id           uuid PRIMARY KEY,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    owner_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    jql          text NOT NULL,
    shared       boolean NOT NULL DEFAULT false,
    version      int NOT NULL
);
CREATE INDEX saved_filters_owner ON saved_filters (workspace_id, owner_id);
