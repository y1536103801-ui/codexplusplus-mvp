#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/codex-plus}"
COMPOSE_FILE="${COMPOSE_FILE:-${DEPLOY_ROOT}/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-${DEPLOY_ROOT}/.env.production}"
PREVIOUS_ENV_FILE="${PREVIOUS_ENV_FILE:-${DEPLOY_ROOT}/.env.production.previous}"

cd "$DEPLOY_ROOT"

if [[ ! -f "$PREVIOUS_ENV_FILE" ]]; then
  echo "Missing previous env file: $PREVIOUS_ENV_FILE" >&2
  exit 1
fi

cp "$ENV_FILE" "${ENV_FILE}.failed.$(date -u +%Y%m%d%H%M%S)"
cp "$PREVIOUS_ENV_FILE" "$ENV_FILE"
chmod 600 "$ENV_FILE"

echo "Rolling back using previous env file..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" pull
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --remove-orphans
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps

echo "Rollback command completed. Run healthcheck.sh next."
