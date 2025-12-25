#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SVC_DIR="$ROOT/services/event-service"
INFRA_COMPOSE="$SVC_DIR/test/integration/infra/compose.yaml"
MIGRATION="$SVC_DIR/migrations/001_init.sql"

# ---- config (override by env if you want) ----
EVENT_BASE_URL="${EVENT_BASE_URL:-http://localhost:8081}"
DATABASE_URL="${DATABASE_URL:-postgres://user:pass@127.0.0.1:5432/app?sslmode=disable}"
JWT_SECRET="${JWT_SECRET:-change_me}"
JWT_ISSUER="${JWT_ISSUER:-auth-service}"

export EVENT_BASE_URL DATABASE_URL JWT_SECRET JWT_ISSUER

# We will prefer reusing your existing "cityevents-*" containers if they are already running.
PG_CONTAINER="${PG_CONTAINER:-cityevents-postgres}"

started_infra=false

echo "===> [1/6] Bring up infra (postgres/redis/rabbitmq) (skip if already running)"

need_up=false
for name in cityevents-postgres cityevents-redis cityevents-rabbitmq; do
  if ! docker ps --format '{{.Names}}' | grep -qx "$name"; then
    need_up=true
  fi
done

if [[ "$need_up" == "true" ]]; then
  echo "Some cityevents-* containers missing -> docker compose up -d (integration infra)"
  docker compose -f "$INFRA_COMPOSE" up -d
  started_infra=true

  # If we started via compose, postgres container might not be named "cityevents-postgres"
  # (depends on whether your compose.yaml uses container_name).
  # Try to find a postgres container we can use.
  if ! docker ps --format '{{.Names}}' | grep -qx "$PG_CONTAINER"; then
    # best effort: pick the first running container with "postgres" in name
    found="$(docker ps --format '{{.Names}}' | grep -i postgres | head -n 1 || true)"
    if [[ -n "$found" ]]; then
      PG_CONTAINER="$found"
    fi
  fi
else
  echo "All cityevents-* containers already running -> skip compose up (reuse dev infra)"
fi

echo "===> [2/6] Wait postgres healthy (container=$PG_CONTAINER)"
for i in {1..60}; do
  if docker exec "$PG_CONTAINER" pg_isready -U user -d app >/dev/null 2>&1; then
    echo "Postgres is ready."
    break
  fi
  sleep 0.5
  if [[ "$i" == "60" ]]; then
    echo "Postgres not ready in time (container=$PG_CONTAINER)" >&2
    docker ps --format "table {{.Names}}\t{{.Status}}" >&2 || true
    exit 1
  fi
done

echo "===> [3/6] Run migration inside docker postgres"
docker exec -i "$PG_CONTAINER" psql -U user -d app < "$MIGRATION" >/dev/null
echo "Migration applied."

echo "===> [4/6] Start event-service (background)"
pushd "$SVC_DIR" >/dev/null
(go run ./api/cmd/main.go) >/tmp/event-service-it.log 2>&1 &
SVC_PID=$!
popd >/dev/null

cleanup() {
  echo "===> cleanup: stopping event-service pid=$SVC_PID"
  kill "$SVC_PID" >/dev/null 2>&1 || true
  wait "$SVC_PID" >/dev/null 2>&1 || true

  if [[ "$started_infra" == "true" ]]; then
    echo "===> cleanup: infra down (only because this script started it)"
    docker compose -f "$INFRA_COMPOSE" down >/dev/null 2>&1 || true
  else
    echo "===> cleanup: infra kept (was already running)"
  fi
}
trap cleanup EXIT

echo "===> [5/6] Wait /healthz on $EVENT_BASE_URL"
for i in {1..80}; do
  if curl -fsS "$EVENT_BASE_URL/healthz" >/dev/null 2>&1; then
    echo "event-service is ready."
    break
  fi
  sleep 0.25
  if [[ "$i" == "80" ]]; then
    echo "event-service not ready. logs:" >&2
    tail -n 160 /tmp/event-service-it.log >&2 || true
    exit 1
  fi
done

echo "===> [6/6] Run integration tests"
pushd "$SVC_DIR" >/dev/null
go test -tags=integration ./test/integration/cases -v
popd >/dev/null

echo "✅ Integration tests done."
