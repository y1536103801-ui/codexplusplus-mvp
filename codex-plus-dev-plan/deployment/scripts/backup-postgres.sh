#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/codex-plus}"
ENV_FILE="${ENV_FILE:-${DEPLOY_ROOT}/.env.production}"
BACKUP_DIR="${BACKUP_DIR:-${DEPLOY_ROOT}/backups/postgres}"

cd "$DEPLOY_ROOT"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"

timestamp="$(date -u +%Y%m%d%H%M%S)"
backup_file="${BACKUP_DIR}/codex_plus_${timestamp}.sql.gz"

echo "Creating PostgreSQL backup: $backup_file"
docker exec codex-plus-postgres pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" | gzip > "$backup_file"
chmod 600 "$backup_file"

retention_days="${BACKUP_RETENTION_DAYS:-7}"
find "$BACKUP_DIR" -type f -name 'codex_plus_*.sql.gz' -mtime +"$retention_days" -delete

echo "Backup completed: $backup_file"
