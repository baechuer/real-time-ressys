# Environment Variables Setup Guide

## Quick Start

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Fill in the required values in `.env`

3. For production, ensure all security settings are configured

## Required Variables

### Must Set (Application won't start without these):

```bash
# JWT Secret - Generate with: openssl rand -base64 32
JWT_SECRET=your-secret-key-here

# RabbitMQ Connection URL
RABBITMQ_URL=amqp://user:password@localhost:5672/
```

## CORS Configuration

### Enable/Disable CORS:

```bash
# Enable CORS (default: true)
CORS_ENABLED=true

# Disable CORS entirely
CORS_ENABLED=false
```

### Configure Allowed Origins:

```bash
# Development - Multiple origins
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080

# Production - Specific domains
CORS_ALLOWED_ORIGINS=https://app.example.com,https://www.example.com

# Wildcard subdomains (matches *.example.com)
CORS_ALLOWED_ORIGINS=*.example.com

# Allow all (NOT recommended for production, defaults if not set)
CORS_ALLOWED_ORIGINS=*
```

**Note:** If `CORS_ALLOWED_ORIGINS` is not set, it defaults to `*` (allow all) for development convenience.

## Security Configuration

### Cookie Security:

```bash
# Option 1: Set environment
ENVIRONMENT=production

# Option 2: Explicit flag
COOKIE_SECURE=true
```

When `ENVIRONMENT=production` or `COOKIE_SECURE=true`, cookies will have `Secure` flag (HTTPS only).

## Complete .env Example

```bash
# Required
JWT_SECRET=your-secret-key-change-in-production
RABBITMQ_URL=amqp://user:password@localhost:5672/

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=social
POSTGRES_SSLMODE=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_DB=0

# Server
ADDR=:8080

# CORS
CORS_ENABLED=true
CORS_ALLOWED_ORIGINS=http://localhost:3000

# Security
ENVIRONMENT=development
COOKIE_SECURE=false
FRONTEND_BASE_URL=http://localhost:8080
```

## Production Checklist

- [ ] Set strong `JWT_SECRET` (use `openssl rand -base64 32`)
- [ ] Set `CORS_ALLOWED_ORIGINS` to exact frontend domains (never use `*`)
- [ ] Set `ENVIRONMENT=production` or `COOKIE_SECURE=true`
- [ ] Ensure HTTPS is enabled
- [ ] Use strong database passwords
- [ ] Use secure RabbitMQ credentials

