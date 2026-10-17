#!/usr/bin/env bash
# Single-host install (Debian/Ubuntu): PostgreSQL, Redis and MinIO run natively under systemd, Kyber runs
# in Docker under a systemd unit (kyber.service). Idempotent: anything already installed or configured is
# left untouched, so re-running only fills in what is missing.
#
#   sudo deploy/install.sh                 # builds the image from this checkout
#   sudo KYBER_IMAGE=ghcr.io/bakhod1r/kyber:latest deploy/install.sh
#
# Settings live in /etc/kyber/kyber.env (written once, never overwritten); edit it and
# `systemctl restart kyber`.
set -euo pipefail

KYBER_IMAGE="${KYBER_IMAGE:-kyber:local}"
KYBER_PORT="${KYBER_PORT:-8080}"
ENV_DIR=/etc/kyber
ENV_FILE="$ENV_DIR/kyber.env"
DATA_DIR=/var/lib/kyber
MINIO_DATA=/var/lib/minio
MINIO_ENV=/etc/default/minio
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

log() { printf '\033[1;32m==>\033[0m %s\n' "$*"; }
skip() { printf '\033[1;33m--\033[0m %s\n' "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }
unit_exists() { systemctl list-unit-files --no-legend "$1.service" 2>/dev/null | grep -q .; }
secret() { head -c 48 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c "${1:-32}"; }

[ "$(id -u)" -eq 0 ] || { echo "run as root (sudo $0)" >&2; exit 1; }
have apt-get || { echo "only Debian/Ubuntu (apt) is supported" >&2; exit 1; }
have systemctl || { echo "systemd is required" >&2; exit 1; }

export DEBIAN_FRONTEND=noninteractive
APT_UPDATED=
apt_install() {
	[ -n "$APT_UPDATED" ] || { apt-get update -qq; APT_UPDATED=1; }
	apt-get install -y -qq "$@"
}
have curl || apt_install curl ca-certificates

case "$(uname -m)" in
x86_64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# --- secrets: reuse the existing env file, generate only on the first run ---------------------------
mkdir -p "$ENV_DIR" "$DATA_DIR/attachments" "$DATA_DIR/recordings"
chown 65532:65532 "$DATA_DIR/attachments" "$DATA_DIR/recordings" # distroless nonroot
if [ -f "$ENV_FILE" ]; then
	skip "$ENV_FILE exists; keeping its settings"
	# shellcheck disable=SC1090
	. "$ENV_FILE"
	DB_PASSWORD="$(sed -n 's#^KYBER_DATABASE_URL=postgres://kyber:\([^@]*\)@.*#\1#p' "$ENV_FILE")"
	S3_ACCESS="${KYBER_S3_ACCESS_KEY:-}"
	S3_SECRET="${KYBER_S3_SECRET_KEY:-}"
	NEW_ENV=
else
	DB_PASSWORD="$(secret 32)"
	S3_ACCESS=kyber
	S3_SECRET="$(secret 40)"
	NEW_ENV=1
fi

# --- PostgreSQL --------------------------------------------------------------------------------------
if have psql && (unit_exists postgresql || pgrep -x postgres >/dev/null); then
	skip "PostgreSQL already installed; not touching it"
else
	log "installing PostgreSQL"
	apt_install postgresql
	systemctl enable --now postgresql
fi
# Kyber connects as a plain role: superusers bypass row-level security (ADR-0010).
if [ -n "$NEW_ENV" ]; then
	if su postgres -c "psql -tAc \"SELECT 1 FROM pg_roles WHERE rolname='kyber'\"" | grep -q 1; then
		echo "role 'kyber' already exists but $ENV_FILE is missing: create $ENV_FILE with its password" >&2
		exit 1
	fi
	log "creating database role and database 'kyber'"
	su postgres -c "psql -v ON_ERROR_STOP=1" <<SQL
CREATE ROLE kyber LOGIN PASSWORD '${DB_PASSWORD}' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
CREATE DATABASE kyber OWNER kyber;
SQL
else
	skip "database role 'kyber' assumed configured"
fi

# --- Redis -------------------------------------------------------------------------------------------
if have redis-server || unit_exists redis-server || unit_exists redis; then
	skip "Redis already installed; not touching it"
else
	log "installing Redis"
	apt_install redis-server
	systemctl enable --now redis-server
fi

# --- MinIO (S3-compatible attachment storage) ---------------------------------------------------------
if have minio || unit_exists minio; then
	skip "MinIO already installed; not touching it"
	[ -n "$S3_ACCESS" ] || skip "add KYBER_S3_* for your MinIO to $ENV_FILE to use it"
else
	log "installing MinIO"
	curl -fsSL -o /usr/local/bin/minio "https://dl.min.io/server/minio/release/linux-$ARCH/minio"
	chmod +x /usr/local/bin/minio
	id minio-user >/dev/null 2>&1 || useradd -r -s /usr/sbin/nologin minio-user
	mkdir -p "$MINIO_DATA" && chown minio-user:minio-user "$MINIO_DATA"
	if [ ! -f "$MINIO_ENV" ]; then
		umask 077
		cat >"$MINIO_ENV" <<EOF
MINIO_ROOT_USER=${S3_ACCESS}
MINIO_ROOT_PASSWORD=${S3_SECRET}
MINIO_VOLUMES=${MINIO_DATA}
MINIO_OPTS="--address 127.0.0.1:9000 --console-address 127.0.0.1:9001"
EOF
		umask 022
	fi
	cat >/etc/systemd/system/minio.service <<'EOF'
[Unit]
Description=MinIO object storage
Wants=network-online.target
After=network-online.target

[Service]
User=minio-user
Group=minio-user
EnvironmentFile=/etc/default/minio
ExecStart=/usr/local/bin/minio server $MINIO_OPTS $MINIO_VOLUMES
Restart=always
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
	systemctl daemon-reload
	systemctl enable --now minio
	log "creating bucket 'kyber'"
	have mc || { curl -fsSL -o /usr/local/bin/mc "https://dl.min.io/client/mc/release/linux-$ARCH/mc"; chmod +x /usr/local/bin/mc; }
	for _ in $(seq 30); do mc alias set kyber-local http://127.0.0.1:9000 "$S3_ACCESS" "$S3_SECRET" >/dev/null 2>&1 && break; sleep 1; done
	mc mb --ignore-existing kyber-local/kyber
fi

# --- Docker ------------------------------------------------------------------------------------------
if have docker; then
	skip "Docker already installed; not touching it"
else
	log "installing Docker"
	curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker

# --- Kyber env file (first run only) -----------------------------------------------------------------
if [ -n "$NEW_ENV" ]; then
	log "writing $ENV_FILE"
	umask 077
	cat >"$ENV_FILE" <<EOF
# Kyber settings (see internal/platform/config). Restart after editing: systemctl restart kyber
KYBER_ADDR=:${KYBER_PORT}
KYBER_DATABASE_URL=postgres://kyber:${DB_PASSWORD}@127.0.0.1:5432/kyber?sslmode=disable
KYBER_REDIS_URL=redis://127.0.0.1:6379/0
KYBER_ENCRYPTION_KEY=$(head -c 32 /dev/urandom | base64)
KYBER_COOKIE_SECURE=false
# KYBER_PUBLIC_URL=https://kyber.example.com
KYBER_LIVEKIT_RECORDINGS_DIR=/data/recordings
EOF
	if [ -n "$S3_ACCESS" ]; then
		cat >>"$ENV_FILE" <<EOF
KYBER_S3_ENDPOINT=127.0.0.1:9000
KYBER_S3_BUCKET=kyber
KYBER_S3_ACCESS_KEY=${S3_ACCESS}
KYBER_S3_SECRET_KEY=${S3_SECRET}
KYBER_S3_TLS=false
EOF
	fi
	umask 022
fi

# --- Kyber image and unit ----------------------------------------------------------------------------
if [ "$KYBER_IMAGE" = kyber:local ]; then
	log "building image $KYBER_IMAGE from $REPO_DIR"
	docker build -t "$KYBER_IMAGE" "$REPO_DIR"
else
	log "pulling $KYBER_IMAGE"
	docker pull "$KYBER_IMAGE"
fi

log "writing /etc/systemd/system/kyber.service"
# Host networking: the container reaches PostgreSQL, Redis and MinIO on 127.0.0.1.
cat >/etc/systemd/system/kyber.service <<EOF
[Unit]
Description=Kyber issue tracker (Docker)
Requires=docker.service
After=docker.service network-online.target postgresql.service redis-server.service minio.service
Wants=network-online.target

[Service]
Restart=always
RestartSec=5
ExecStartPre=-/usr/bin/docker rm -f kyber
ExecStart=/usr/bin/docker run --rm --name kyber --network host \\
  --env-file ${ENV_FILE} \\
  -v ${DATA_DIR}/attachments:/data/attachments \\
  -v ${DATA_DIR}/recordings:/data/recordings \\
  ${KYBER_IMAGE}
ExecStop=/usr/bin/docker stop -t 20 kyber

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable kyber
systemctl restart kyber

log "done: Kyber on http://$(hostname -I 2>/dev/null | awk '{print $1}'):${KYBER_PORT}"
echo "   logs: journalctl -u kyber -f    settings: $ENV_FILE"
