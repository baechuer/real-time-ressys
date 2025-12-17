include .env
MIGRATION_DIR=migrations
DB_ADDR=postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=$(POSTGRES_SSLMODE)

.PHONY: migrate-up migrate-down migration
migration:
	@migrate create -seq -ext sql -dir $(MIGRATION_DIR) $(filter-out $@,$(MAKECMDGOALS))

migrate-up:
	@migrate -path=$(MIGRATION_DIR) -database $(DB_ADDR) up

migrate-down:
	@migrate -path=$(MIGRATION_DIR) -database $(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

# Testing targets
.PHONY: test test-unit test-integration test-e2e test-coverage test-coverage-html test-race test-bench

# Run all tests
test:
	@go test -v ./...
	@echo "Cleaning up test executables..."
	@rm -f *.test.exe *.test *.out 2>/dev/null || true
	@find . -name "*.test.exe" -delete 2>/dev/null || true
	@find . -name "*.test" -delete 2>/dev/null || true

# Run unit tests only
test-unit:
	@go test -v ./app/...

# Run integration tests only
test-integration:
	@go test -v ./tests/integration/...

# Run E2E tests only
test-e2e:
	@go test -v ./tests/e2e/...

# Generate coverage report (terminal)
test-coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# Generate HTML coverage report
test-coverage-html:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
test-race:
	@go test -race ./...

# Run benchmarks
test-bench:
	@go test -bench=. -benchmem ./...