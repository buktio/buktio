#!/usr/bin/env bash
# Run the buktio API locally against the dev infra (docker-compose.dev.yml).
# Loads secrets from deploy/docker-compose/.env and points the API at the
# host-published Postgres (:5432) and Garage (:3900 / :3903).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT/deploy/docker-compose/.env"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE — run deploy/docker-compose/gen-env.sh first." >&2
  exit 1
fi

# Load .env (KEY=VALUE lines).
set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

# Host ports must match docker-compose.dev.yml (override via .env if changed).
PG_PORT="${BUKTIO_PG_PORT:-5433}"
S3_PORT="${BUKTIO_S3_PORT:-3900}"
ADMIN_PORT="${BUKTIO_ADMIN_PORT:-3903}"

export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${PG_PORT}/${POSTGRES_DB}?sslmode=disable"
export GARAGE_ADMIN_URL="http://localhost:${ADMIN_PORT}"
export GARAGE_S3_URL="http://localhost:${S3_PORT}"
export GARAGE_S3_REGION="garage"
export BUKTIO_HTTP_ADDR="${BUKTIO_HTTP_ADDR:-:8080}"
export BUKTIO_LOG_LEVEL="${BUKTIO_LOG_LEVEL:-info}"

cd "$ROOT"
echo "Starting buktio API on ${BUKTIO_HTTP_ADDR} (DB :5432, storage :3900/:3903)..."
exec go run ./apps/api/cmd/server
