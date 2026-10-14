#!/bin/sh
# Runs once, when the database volume is created: Kyber connects as a plain role, because
# superusers bypass row-level security (ADR-0010). The postgres superuser stays for admin work.
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
CREATE ROLE kyber LOGIN PASSWORD '${KYBER_DB_PASSWORD}' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE DATABASE kyber OWNER kyber;
SQL
