# City Events Platform - BFF API Contract (v0.1)

> This document defines the **edge contract** exposed by the **Next.js BFF** to the **Browser**.
> The Browser **MUST NOT** call usage microservices directly. All traffic is proxied/aggregated by `/api/*`.

---

## 1. Unified Error Format
All API responses with HTTP status >= 400 MUST follow this structure.

```json
{
  "error": {
    "code": "resource_not_found",    // Stable error code (snake_case)
    "message": "Event not found",    // Human readable message (safe for UI toast)
    "request_id": "req-123-abc",     // For tracing/debugging
    "meta": {                        // Optional context
      "field": "email"
    }
  }
}
```

### Common Error Codes
| Code | HTTP | Description | UI Handling |
| :--- | :--- | :--- | :--- |
| `unauthorized` | 401 | Not logged in / Session expired | Auto-refresh or redirect to Login |
| `forbidden` | 403 | Logged in but no permission | Show "Access Denied" page/state |
| `validation_failed` | 400 | Bad input (see `meta`) | Show form errors |
| `conflict_state` | 409 | e.g. Already joined, Event full | Show error toast |
| `internal_error` | 500/502 | Server or Upstream error | Show generic "Try again later" |
| `upstream_timeout` | 504 | Service slow/down | Show retry toast |

---

## 2. Pagination (Cursor-based)
Used for Feeds and Lists.

### Request Parameters
*   `cursor`: (Optional) Opaque string from previous response `next_cursor`. If empty, start from beginning.
*   `limit`: (Optional) Number of items (default 20, max 50).

### Response Envelope
```json
{
  "items": [ ... ],       // Array of ViewModels
  "next_cursor": "...",   // Opaque string. NULL if no more pages.
  "has_more": true        // Convenience boolean
}
```

---

## 3. Endpoints

### 3.1 Auth
*   `POST /api/auth/register` (Proxy -> `auth-service`)
*   `POST /api/auth/login` (Proxy -> `auth-service`; Set-Cookie)
*   `POST /api/auth/refresh` (Proxy -> `auth-service`; Rotate Cookie)
*   `POST /api/auth/logout` (Proxy -> `auth-service`; Clear Cookie)
*   `GET /api/auth/me` (Proxy -> `auth-service`; Get User)

### 3.2 Feed & Events
#### `GET /api/feed` & `GET /api/events`
Both endpoints share **identical pagination guarantees**.
*   **Params**: `cursor` (opaque), `limit` (max 50), `category`, `time_range`.
*   **Sorting**:
    *   `/events`: Strict chronological (`start_time ASC, id ASC`).
    *   `/feed`: Relevance/Discovery (V0: proxied `event-service` sort).
*   **Response**:
    ```json
    {
      "items": [ ... ],
      "next_cursor": "eyJ...", // NULL if end
      "has_more": true
    }
    ```

#### `GET /api/events/{id}/view`
The single event page aggregation.
*   **Source**: Aggregates `event-service` + `join-service`.
*   **Degradation**: `participation` is `null` **only** if `degraded.participation` is set.
*   **Response (`EventView`)**:
    ```json
    {
      "event": { ... },
      "participation": {        // strict schema: object | null
        "status": "none" | "joined" | "canceled" | "waitlisted",
        "joined_at": "2024-01-01T12:00:00Z"
      },
      "actions": {              // computed logic
        "can_join": true,       // Computed by ActionPolicy (Auth > Degraded > Business)
        "can_cancel": false,
        "reason": "success" | "auth_required" | "participation_unavailable" | "event_full"
      },
      "degraded": {
        "participation": "timeout" | "unavailable" | "rate_limited" | null
      }
    }
    ```

### 3.3 Actions
#### `POST /api/events/{id}/join`
#### `POST /api/events/{id}/cancel`
*   **Header**: `Idempotency-Key` (Required). **Missing = 400**.
*   **Retry Policy**: **BFF DOES NOT auto-retry**.
*   **Source**: Proxies to `join-service`.
*   **Response**:
    *   `200 OK`: Action succeeded (or `state_already_reached`).
        *   **Body**: `JoinState` (e.g. `{ "status": "joined", "event_id": "...", "user_id": "..." }`)
    *   `409 Conflict`: `idempotency_key_mismatch` (Critical).
    *   `400 Bad Request`: Validation failure.

---

## 4. View Models (Frontend Types)

### `EventCard` (List Item)
Optimized for rendering cards.
```typescript
interface EventCard {
  id: string;
  title: string;
  cover_image?: string; // Placeholder for now
  start_time: string;   // ISO8601
  city: string;
  category: string;
  score?: number;       // For debug/sorting
}
```

### `EventView` (Detail Page)
Includes full description and participation context.
```typescript
interface EventView {
  event: EventCard & {
    description: string;
    capacity: number;
    filled_count: number; // Statistic if available
    organizer_id: string;
  };
  viewer_context: {
    participation_status: 'none' | 'active' | 'waitlisted' | 'cancelled';
  };
}
```
auth service.env:

APP_ENV=dev
HTTP_ADDR=:8080

JWT_SECRET=change_me
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h

DB_ADDR=postgres://user:pass@127.0.0.1:5432/app?sslmode=disable

# --- Redis (TokenVersion cache etc.) ---
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
TOKENVER_CACHE_TTL=10m
REDIS_ENABLED=true

RABBIT_URL=amqp://guest:guest@localhost:5672/
HTTP_READ_TIMEOUT=10s
HTTP_WRITE_TIMEOUT=90s
HTTP_IDLE_TIMEOUT=60s

LOG_LEVEL=debug
LOG_FORMAT=console   # In Production change it to json
VERIFY_EMAIL_BASE_URL=http://localhost:8080/auth/v1/verify-email/confirm?token=
PASSWORD_RESET_BASE_URL=http://localhost:8080/auth/v1/password/reset/validate?token=
#VERIFY_EMAIL_BASE_URL=https://api.cityevents.com/auth/v1/verify-email/confirm?token=
#PASSWORD_RESET_BASE_URL=https://api.cityevents.com/auth/v1/password/reset/validate?token=
RABBIT_URL=amqp://guest:guest@localhost:5672/

VERIFY_EMAIL_TOKEN_TTL=24h
PASSWORD_RESET_TOKEN_TTL=30m
POSTGRES_USER=user
POSTGRES_PASSWORD=pass
POSTGRES_DB=app
POSTGRES_PORT=5432
DB_DEBUG=false
AUTH_DEV_ECHO_TOKENS=true
INTERNAL_SECRET_KEY=sharedkey

bff:
# Server Port
HTTP_PORT=8080

# Microservices URLs (Docker DNS names by default)
# If running locally without docker-compose, use http://localhost:8081, etc.
AUTH_SERVICE_URL=http://auth-service:8080
EVENT_SERVICE_URL=http://event-service:8080
JOIN_SERVICE_URL=http://join-service:8080
# Local Debugging Mode
# AUTH_SERVICE_URL=http://localhost:8081
# EVENT_SERVICE_URL=http://localhost:8082
# JOIN_SERVICE_URL=http://localhost:8083 email service
RABBIT_URL=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=city.events
RABBIT_QUEUE=email-service.q
RABBIT_BIND_KEYS=auth.email.#,auth.password.#
RABBIT_PREFETCH=10
RABBIT_CONSUMER_TAG=email-service
LOG_LEVEL=info
LOG_FORMAT=console
LOG_COLOR=0
LOG_CALLER=1
# LOG_TIME_FORMAT=2006-01-02T15:04:05Z07:00  # 可不填，默认 RFC3339
FAKE_FAIL_MODE=none
# EMAIL_MAX_ATTEMPTS=5
# 让 circuit breaker 不要太快开（本地演示可先高一点）
#EMAIL_CB_THRESHOLD=999
# choose sender: fake | smtp
EMAIL_SENDER=fake

SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=baechuer@gmail.com
SMTP_PASSWORD=tzzo qrsi hftx frfn   # Gmail App Password
SMTP_FROM=City Event Platform <baechuer@gmail.com>

# optional
SMTP_TIMEOUT=10s

# HTML server
EMAIL_WEB_ADDR=:8090
AUTH_BASE_URL=http://localhost:8080
AUTH_VERIFY_CONFIRM_PATH=/auth/v1/verify-email/confirm
AUTH_RESET_VALIDATE_PATH=/auth/v1/password/reset/validate
AUTH_RESET_CONFIRM_PATH=/auth/v1/password/reset/confirm
EMAIL_PUBLIC_BASE_URL=http://localhost:8090
#Redis

REDIS_ENABLED=true
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
EMAIL_IDEMPOTENCY_TTL=24h
# --- HTTP/API rate limiting (email-service web :8090) ---
RL_ENABLED=true

# per-IP limit (per endpoint)
RL_IP_LIMIT=30
RL_IP_WINDOW=1m

# per-token limit (per endpoint)
RL_TOKEN_LIMIT=10
RL_TOKEN_WINDOW=10m
INTERNAL_SECRET_KEY=sharedkey

event service:
# --- Service Base Settings ---
APP_ENV=dev
HTTP_ADDR=:8081
# Recommended for local development in Sydney
TZ=Australia/Sydney

# --- PostgreSQL Database ---
# Format: postgres://user:password@host:port/dbname?sslmode=disable
# Ensure you create the 'cityevents_event' database in your Postgres instance
DATABASE_URL=postgres://user:pass@127.0.0.1:5432/app?sslmode=disable
POSTGRES_USER=user
POSTGRES_PASSWORD=pass
POSTGRES_DB=app
POSTGRES_PORT=5432
DB_DEBUG=false

# --- Authentication (JWT Local Validation) ---
# IMPORTANT: This MUST match the JWT_SECRET used in your auth-service
JWT_SECRET=change_me
JWT_ISSUER=auth-service

# --- Redis (Blacklist & Rate Limiting) ---
REDIS_ENABLED=true
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
# Use a different DB index than email-service (e.g., DB 1) to avoid key collision
REDIS_DB=1

# --- Rate Limiting (Event API) ---
RL_ENABLED=true
RL_IP_LIMIT=100
RL_IP_WINDOW=1m

# --- RabbitMQ (Event Publisher) ---
# Used to publish "event.created" or "event.published" messages
RABBIT_URL=amqp://guest:guest@localhost:5672/
RABBIT_EXCHANGE=city.events

# --- Logger ---
LOG_LEVEL=debug
LOG_FORMAT=console
join servcice:

# -------- Runtime --------
APP_ENV=dev
DEBUG=true
PORT=8083
TZ=Australia/Sydney

# -------- PostgreSQL --------
POSTGRES_ADDR=127.0.0.1:5432
POSTGRES_USER=user
POSTGRES_PASSWORD=pass
POSTGRES_DB=app
POSTGRES_SSLMODE=disable

# -------- JWT --------
JWT_SECRET=change_me
JWT_ISSUER=auth-service

# -------- Redis --------
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=
REDIS_DB=2

# -------- Cache --------
CACHE_USER_TTL=10m

# -------- Rate limit --------
RL_ENABLED=true
RL_REQUESTS_LIMIT=100
RL_WINDOW_SECONDS=60

# -------- RabbitMQ --------
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=city.events

# -------- Logging --------
LOG_LEVEL=debug


is it because ur config was wrong?