# API Documentation Guide

## How to View the API Specification

### Option 1: Direct JSON Endpoint (Recommended)

Once your server is running, access the OpenAPI specification at:

```
GET http://localhost:8080/auth/v1/openapi.json
```

**Example using curl:**
```bash
curl http://localhost:8080/auth/v1/openapi.json | jq
```

### Option 2: Swagger UI (Interactive)

1. **Install Swagger UI** (if not already installed):
   ```bash
   npm install -g swagger-ui-serve
   ```

2. **Download the OpenAPI spec:**
   ```bash
   curl http://localhost:8080/auth/v1/openapi.json -o openapi.json
   ```

3. **Serve with Swagger UI:**
   ```bash
   swagger-ui-serve openapi.json
   ```
   
   Or use Docker:
   ```bash
   docker run -p 8081:8080 -e SWAGGER_JSON=/openapi.json -v $(pwd)/openapi.json:/openapi.json swaggerapi/swagger-ui
   ```

4. **Access Swagger UI:**
   ```
   http://localhost:8081
   ```

### Option 3: Postman

1. Open Postman
2. Click **Import**
3. Select **Link** tab
4. Enter: `http://localhost:8080/auth/v1/openapi.json`
5. Click **Continue** and **Import**

### Option 4: Online Swagger Editor

1. Go to https://editor.swagger.io/
2. Click **File** → **Import file**
3. Enter URL: `http://localhost:8080/auth/v1/openapi.json`
4. Or paste the JSON directly

### Option 5: Redoc

1. **Install Redoc CLI:**
   ```bash
   npm install -g redoc-cli
   ```

2. **Generate HTML documentation:**
   ```bash
   curl http://localhost:8080/auth/v1/openapi.json -o openapi.json
   redoc-cli bundle openapi.json -o api-docs.html
   ```

3. **Open in browser:**
   ```bash
   open api-docs.html
   ```

## Available Endpoints

### Health & Monitoring
- `GET /auth/v1/health` - Health check with dependency status
- `GET /auth/v1/metrics` - Prometheus metrics
- `GET /auth/v1/openapi.json` - OpenAPI specification

### Authentication
- `POST /auth/v1/register` - Register new user
- `POST /auth/v1/login` - Login and get tokens
- `POST /auth/v1/logout` - Logout (revoke tokens)
- `POST /auth/v1/refresh` - Refresh access token

### Email & Password
- `POST /auth/v1/verify-email` - Verify email address
- `POST /auth/v1/request-password-reset` - Request password reset (authenticated)
- `POST /auth/v1/reset-password` - Reset password with token

### User
- `GET /auth/v1/me` - Get current user info (protected)
- `GET /auth/v1/admin` - Admin-only endpoint (protected)

## Testing the API

### Start the Server

```bash
# Make sure dependencies are running
docker-compose up -d

# Set required environment variables
export JWT_SECRET=your-secret-key-here
export RABBITMQ_URL=amqp://user:password@localhost:5672/

# Run the server
go run ./app/handlers
```

### View OpenAPI Spec

```bash
# Get the spec
curl http://localhost:8080/auth/v1/openapi.json

# Pretty print with jq
curl http://localhost:8080/auth/v1/openapi.json | jq

# Save to file
curl http://localhost:8080/auth/v1/openapi.json -o openapi.json
```

## Example API Calls

### Health Check
```bash
curl http://localhost:8080/auth/v1/health | jq
```

### Register User
```bash
curl -X POST http://localhost:8080/auth/v1/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123",
    "username": "johndoe"
  }'
```

### Login
```bash
curl -X POST http://localhost:8080/auth/v1/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123"
  }'
```

### Get Current User (Protected)
```bash
curl http://localhost:8080/auth/v1/me \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## API Specification Details

The OpenAPI specification includes:
- ✅ All endpoint definitions
- ✅ Request/response schemas
- ✅ Authentication requirements
- ✅ Error responses
- ✅ Example values

The spec is automatically generated and always reflects the current API implementation.

