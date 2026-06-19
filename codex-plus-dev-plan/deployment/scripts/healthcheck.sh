#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/codex-plus}"
ENV_FILE="${ENV_FILE:-${DEPLOY_ROOT}/.env.production}"

cd "$DEPLOY_ROOT"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

api_url="${PUBLIC_BASE_URL%/}/health"
admin_url="${ADMIN_PUBLIC_URL%/}"

echo "Checking API: $api_url"
curl --fail --silent --show-error --max-time 10 "$api_url" >/dev/null

echo "Checking admin: $admin_url"
curl --fail --silent --show-error --max-time 10 "$admin_url" >/dev/null

echo "Checking containers..."
docker ps --filter "name=codex-plus-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo "Healthcheck passed."
