#!/usr/bin/env bash
set -euo pipefail

DEPLOY_ROOT="${DEPLOY_ROOT:-/opt/codex-plus}"
COMPOSE_FILE="${COMPOSE_FILE:-${DEPLOY_ROOT}/docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-${DEPLOY_ROOT}/.env.production}"
PREVIOUS_ENV_FILE="${PREVIOUS_ENV_FILE:-${DEPLOY_ROOT}/.env.production.previous}"

cd "$DEPLOY_ROOT"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "Missing compose file: $COMPOSE_FILE" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing env file: $ENV_FILE" >&2
  exit 1
fi

if grep -q "REPLACE_ME\\|REPLACE_WITH_SERVER_SECRET" "$ENV_FILE"; then
  echo "Env file still contains placeholder values." >&2
  exit 1
fi

cp "$ENV_FILE" "$PREVIOUS_ENV_FILE"
chmod 600 "$ENV_FILE" "$PREVIOUS_ENV_FILE"

echo "Pulling images..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" pull

echo "Starting services..."
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d --remove-orphans

echo "Current service status:"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps

echo "Deployment command completed. Run healthcheck.sh next."
