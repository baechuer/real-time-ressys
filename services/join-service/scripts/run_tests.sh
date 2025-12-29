#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 Starting Test Infrastructure..."
docker-compose -f "$PROJECT_ROOT/docker-compose.test.yml" up -d

echo "⏳ Waiting for Postgres..."
until docker exec join_service_test_db pg_isready -U test_user > /dev/null 2>&1; do
  echo "   ...waiting for db to be ready"
  sleep 1
done
echo "   DB is ready!"

echo "🛠 Applying Migrations..."

for file in "$PROJECT_ROOT"/migrations/*.sql; do
    echo "   -> Applying $(basename "$file")..."
    docker exec -i join_service_test_db \
        psql -U test_user -d join_service_test < "$file" > /dev/null
done

echo "🧪 Running Tests..."
export TEST_DB_DSN="postgres://test_user:test_password@localhost:5433/join_service_test?sslmode=disable"
export TEST_REDIS_ADDR="localhost:6380"
export TEST_RABBIT_URL="amqp://guest:guest@localhost:5674/"
export TEST_RABBIT_EXCHANGE="city.events.test"

go test -v -p 1 -count=1 "$PROJECT_ROOT/internal/..."

echo "✅ Tests Passed!"
docker-compose -f docker-compose.test.yml down -v