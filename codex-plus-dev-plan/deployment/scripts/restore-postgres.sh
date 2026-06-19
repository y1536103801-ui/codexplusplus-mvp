#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/codex-plus}"
ENV_FILE="${ENV_FILE:-${DEPLOY_ROOT}/.env.production}"
BACKUP_FILE="${1:-}"

if [[ -z "$BACKUP_FILE" ]]; then
  echo "Usage: restore-postgres.sh <backup.sql.gz>" >&2
  exit 1
fi

if [[ ! -f "$BACKUP_FILE" ]]; then
  echo "Backup file not found: $BACKUP_FILE" >&2
  exit 1
fi

cd "$DEPLOY_ROOT"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

echo "This restore will overwrite data in database '$POSTGRES_DB' inside container codex-plus-postgres."
echo "Set CONFIRM_RESTORE=YES to continue."

if [[ "${CONFIRM_RESTORE:-}" != "YES" ]]; then
  echo "Restore aborted." >&2
  exit 1
fi

gunzip -c "$BACKUP_FILE" | docker exec -i codex-plus-postgres psql -U "$POSTGRES_USER" "$POSTGRES_DB"
echo "Restore completed."
