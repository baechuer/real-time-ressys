#!/usr/bin/env bash
set -euo pipefail

docker compose -f test/integration/infra/compose.yaml up -d

# optional: show status
docker compose -f test/integration/infra/compose.yaml ps

go test -tags=integration ./test/integration/... -count=1 -p=1

docker compose -f test/integration/infra/compose.yaml down -v
