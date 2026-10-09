CREATE TABLE projects (
    id         uuid PRIMARY KEY,
    key        text NOT NULL UNIQUE CHECK (key ~ '^[A-Z][A-Z0-9]{1,9}$'),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    issue_seq  integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE issues (
    id          uuid PRIMARY KEY,
    project_key text NOT NULL REFERENCES projects (key),
    number      integer NOT NULL CHECK (number > 0),
    title       text NOT NULL,
    type        text NOT NULL,
    status      text NOT NULL,
    version     integer NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_key, number)
);
CREATE INDEX issues_project_status ON issues (project_key, status);

-- Transactional outbox: written in the same transaction as the aggregate change.
CREATE TABLE outbox (
    id           bigserial PRIMARY KEY,
    name         text NOT NULL,
    payload      jsonb NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz
);
CREATE INDEX outbox_unpublished ON outbox (id) WHERE published_at IS NULL;

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user ON sessions (user_id);
