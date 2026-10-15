-- S107b (ADR-0018 §4): email domains an SSO connection serves. A domain routes sign-in only once
-- verified (DNS TXT "kyber-verify=<token>", or trusted by a site admin); one connection per domain.
CREATE TABLE sso_connection_domains (
    name          text PRIMARY KEY,
    connection_id uuid    NOT NULL REFERENCES sso_connections (id) ON DELETE CASCADE,
    token         text    NOT NULL,
    verified      boolean NOT NULL DEFAULT false
);
CREATE INDEX sso_connection_domains_connection ON sso_connection_domains (connection_id);
