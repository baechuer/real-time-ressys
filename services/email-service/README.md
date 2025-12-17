# Email Service

A microservice worker that consumes email events from RabbitMQ and sends emails via various providers (SendGrid, AWS SES, SMTP).

## Architecture

This is a **pure worker service** - it does not expose public HTTP endpoints (except health checks). It consumes messages from RabbitMQ and sends emails asynchronously.

See [EMAIL_SERVICE_ARCHITECTURE.md](./EMAIL_SERVICE_ARCHITECTURE.md) for detailed architecture documentation.

## Features

- ✅ RabbitMQ consumer with worker pool
- ✅ Idempotency (prevents duplicate emails)
- ✅ Retry mechanism with exponential backoff
- ✅ Dead Letter Queue (DLQ) for failed messages
- ✅ Multiple email provider support (SendGrid, AWS SES, SMTP)
- ✅ Email template rendering
- ✅ Request ID tracing (distributed tracing)
- ✅ Structured logging
- ✅ Prometheus metrics
- ✅ Health checks
- ✅ Graceful shutdown

## Quick Start

### Prerequisites

- Go 1.25+
- RabbitMQ (running)
- Redis (for idempotency)
- Email provider account (SendGrid/SES/SMTP)

### Setup

1. **Copy environment file**:
   ```bash
   cp .env.example .env
   ```

2. **Configure environment variables** (see `.env.example`)

3. **Install dependencies**:
   ```bash
   go mod download
   ```

4. **Run locally**:
   ```bash
   go run app/main.go
   ```

### Using Docker Compose

```bash
docker-compose up -d
```

## Configuration

See `.env.example` for all available configuration options.

### Required Variables

- `RABBITMQ_URL` - RabbitMQ connection string
- `EMAIL_PROVIDER` - One of: `sendgrid`, `ses`, `smtp`
- Provider-specific credentials (see `.env.example`)

## Message Format

The service consumes messages from `auth.events` exchange with the following formats:

### Email Verification

```json
{
  "type": "email_verification",
  "email": "user@example.com",
  "verification_url": "https://frontend.com/verify-email?token=..."
}
```

### Password Reset

```json
{
  "type": "password_reset",
  "email": "user@example.com",
  "reset_url": "https://frontend.com/reset-password?token=..."
}
```

**Note**: Message format matches `auth-service` exactly. See `app/models/message.go`.

## Development

### Run Tests

```bash
# Unit tests
make test-unit

# Integration tests (requires Docker)
make test-integration

# All tests
make test

# With coverage
make test-coverage
```

### Local Development

1. Start dependencies:
   ```bash
   docker-compose up -d rabbitmq redis
   ```

2. Run the service:
   ```bash
   go run app/main.go
   ```

3. Send test message (using RabbitMQ management UI or CLI)

## Project Structure

```
app/
  ├── config/        # Configuration (env, RabbitMQ, Redis, email)
  ├── consumer/      # RabbitMQ consumer and handlers
  ├── email/         # Email sending and templates
  ├── idempotency/   # Idempotency checking (Redis)
  ├── retry/         # Retry logic and DLQ
  ├── models/        # Data models
  ├── logger/        # Structured logging
  ├── metrics/       # Prometheus metrics
  ├── errors/        # Error definitions
  └── main.go        # Entry point
```

See [EMAIL_SERVICE_ARCHITECTURE.md](./EMAIL_SERVICE_ARCHITECTURE.md) for detailed structure.

## Health Checks

- `GET /health` - Basic health check
- `GET /health/rabbitmq` - RabbitMQ connection status
- `GET /health/redis` - Redis connection status
- `GET /health/email` - Email provider connectivity

## Metrics

Prometheus metrics available at `/metrics`:
- `email_messages_consumed_total`
- `email_sent_total`
- `email_failed_total`
- `email_retry_total`
- `email_dlq_total`
- `email_processing_duration_seconds`

## Deployment

The service is stateless and can be scaled horizontally:

```bash
# Build
docker build -t email-service .

# Run multiple instances
docker run -d email-service
docker run -d email-service
docker run -d email-service
```

Each instance will consume messages from the same queues, providing load distribution.

## License

[Your License Here]

