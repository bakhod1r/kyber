#!/usr/bin/env bash
# dev-env.sh — the local tools Kyber's tests need, without installing anything system-wide.
#
#   scripts/dev-env.sh up      start a throwaway PostgreSQL 16 on :55432 (Docker, else local binaries)
#   scripts/dev-env.sh down    stop it
#   scripts/dev-env.sh env     print the export line:  eval "$(scripts/dev-env.sh env)"
#   scripts/dev-env.sh k6      download k6 (pinned) into .dev/bin for loadtest/api.js
#
# The database trusts local connections and holds test data only; never point it at real data.
set -euo pipefail

PORT=${KYBER_DEV_PG_PORT:-55432}
DIR=${KYBER_DEV_DIR:-"$(cd "$(dirname "$0")/.." && pwd)/.dev"}
CONTAINER=kyber-dev-pg
K6_VERSION=${K6_VERSION:-v0.57.0}
URL="postgres://postgres@localhost:${PORT}/kyber_test?sslmode=disable"

has() { command -v "$1" >/dev/null 2>&1; }
ready() { has pg_isready && pg_isready -q -h localhost -p "$PORT"; }

# pgbin finds PostgreSQL's server binaries (Debian keeps them outside PATH).
pgbin() {
	if has pg_ctl; then dirname "$(command -v pg_ctl)"; return; fi
	ls -d /usr/lib/postgresql/*/bin 2>/dev/null | sort -V | tail -n1
}

# as_pg runs a command as the postgres OS user when we are root (initdb refuses root).
as_pg() {
	if [ "$(id -u)" = 0 ]; then su postgres -c "$*"; else sh -c "$*"; fi
}

up_docker() {
	if docker ps --format '{{.Names}}' | grep -qx "$CONTAINER"; then return; fi
	docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
	docker run -d --name "$CONTAINER" -p "${PORT}:5432" -e POSTGRES_HOST_AUTH_METHOD=trust \
		-e POSTGRES_DB=kyber_test postgres:16-alpine >/dev/null
	for _ in $(seq 30); do docker exec "$CONTAINER" pg_isready -q -U postgres && return; sleep 1; done
	echo "PostgreSQL did not start" >&2; exit 1
}

up_local() {
	local bin data="$DIR/pg/data"
	bin=$(pgbin)
	[ -n "$bin" ] || { echo "Neither Docker nor PostgreSQL server binaries found; install one of them." >&2; exit 1; }
	mkdir -p "$DIR/pg"
	[ "$(id -u)" = 0 ] && chown -R postgres "$DIR/pg"
	if [ ! -f "$data/PG_VERSION" ]; then
		as_pg "'$bin/initdb' -D '$data' -U postgres --auth=trust" >/dev/null
	fi
	ready || as_pg "'$bin/pg_ctl' -D '$data' -l '$DIR/pg/log' -o '-p $PORT -k /tmp' -w start" >/dev/null
	as_pg "'$bin/psql' -h localhost -p $PORT -U postgres -tAc \"SELECT 1 FROM pg_database WHERE datname = 'kyber_test'\"" | grep -q 1 ||
		as_pg "'$bin/createdb' -h localhost -p $PORT -U postgres kyber_test"
}

case "${1:-}" in
up)
	if ready; then echo "PostgreSQL already answers on :$PORT" >&2
	elif has docker && docker info >/dev/null 2>&1; then up_docker
	else up_local
	fi
	echo "export KYBER_TEST_DATABASE_URL=\"$URL\""
	;;
down)
	if has docker && docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx "$CONTAINER"; then docker rm -f "$CONTAINER" >/dev/null
	elif [ -f "$DIR/pg/data/PG_VERSION" ]; then as_pg "'$(pgbin)/pg_ctl' -D '$DIR/pg/data' -m fast stop" >/dev/null
	fi
	;;
env) echo "export KYBER_TEST_DATABASE_URL=\"$URL\"" ;;
k6)
	if [ ! -x "$DIR/bin/k6" ]; then
		mkdir -p "$DIR/bin"
		arch=$(uname -m); [ "$arch" = x86_64 ] && arch=amd64; [ "$arch" = aarch64 ] && arch=arm64
		os=$(uname -s | tr '[:upper:]' '[:lower:]')
		curl -fsSL "https://github.com/grafana/k6/releases/download/${K6_VERSION}/k6-${K6_VERSION}-${os}-${arch}.tar.gz" |
			tar -xz -C "$DIR/bin" --strip-components=1 "k6-${K6_VERSION}-${os}-${arch}/k6"
	fi
	echo "$DIR/bin/k6"
	;;
*) sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
