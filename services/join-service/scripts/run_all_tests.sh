#!/bin/bash
set -euo pipefail


SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
docker-compose -f "$PROJECT_ROOT/docker-compose.test.yml" down -v || true
docker-compose -f "$PROJECT_ROOT/docker-compose.test.yml" up -d
echo "🧪 [1/2] Unit tests (no docker)..."
(
  cd "$PROJECT_ROOT"
  go test ./... -race
)

echo ""
echo "🧪 [2/2] Integration tests (postgres+redis+rabbit via docker-compose)..."
docker-compose -f "$PROJECT_ROOT/docker-compose.test.yml" up -d

echo "⏳ Waiting for Postgres..."
until docker exec join_service_test_db pg_isready -U test_user > /dev/null 2>&1; do
  sleep 1
done
echo "✅ Postgres is ready"

echo "⏳ Waiting for Redis..."
until docker exec join_service_test_redis redis-cli ping > /dev/null 2>&1; do
  sleep 1
done
echo "✅ Redis is ready"

echo "⏳ Waiting for RabbitMQ..."
until docker exec join_service_test_rabbit rabbitmq-diagnostics -q ping > /dev/null 2>&1; do
  sleep 1
done
echo "✅ RabbitMQ is ready"

echo "🛠 Applying Migrations..."
for file in "$PROJECT_ROOT"/migrations/*.sql; do
  echo "   -> $(basename "$file")"
  docker exec -i join_service_test_db psql -U test_user -d join_service_test < "$file" > /dev/null
done

export TEST_DB_DSN="postgres://test_user:test_password@localhost:5433/join_service_test?sslmode=disable"
export TEST_REDIS_ADDR="localhost:6380"
export TEST_RABBIT_URL="amqp://test_user:test_password@localhost:5673/"
export TEST_RABBIT_EXCHANGE="city.events.test"

echo "🧪 Running integration test packages..."
(
  cd "$PROJECT_ROOT"
  go test -tags=integration -v -p 1 -count=1 ./internal/...
)

echo "✅ All tests passed"
docker-compose -f "$PROJECT_ROOT/docker-compose.test.yml" down -v
