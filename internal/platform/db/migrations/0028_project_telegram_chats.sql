-- Telegram groups subscribed to a project's events, and the one-time startgroup links that connect them.
CREATE TABLE telegram_group_tokens (
    hash         text PRIMARY KEY,
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL
);
CREATE TABLE project_telegram_chats (
    workspace_id uuid NOT NULL,
    project_key  text NOT NULL,
    chat_id      text NOT NULL,
    title        text NOT NULL DEFAULT '',
    linked_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    linked_at    timestamptz NOT NULL,
    PRIMARY KEY (workspace_id, project_key, chat_id),
    FOREIGN KEY (workspace_id, project_key) REFERENCES projects (workspace_id, key) ON DELETE CASCADE
);
