include .env

.PHONY: run test test-unit test-integration test-e2e test-coverage test-coverage-html test-race test-bench

# Run the email service
run:
	@go run app/main.go

# Testing targets
test:
	@go test -v ./...

test-unit:
	@go test -v -short ./app/...

test-integration:
	@go test -v ./tests/integration/...

test-coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

test-coverage-html:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-e2e:
	@go test -v ./tests/e2e/...

test-race:
	@go test -race -v ./...

test-bench:
	@go test -bench=. -benchmem ./...

# Development
deps:
	@go mod download
	@go mod tidy

# Docker
docker-build:
	@docker build -t email-service .

docker-run:
	@docker-compose up -d

docker-down:
	@docker-compose down

