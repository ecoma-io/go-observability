# Gin Service Example

A Gin-based HTTP service with full observability middleware integration.

## Features

- Gin framework with observability middleware
- Automatic request logging with trace context
- Panic recovery with structured error responses
- Trace ID propagation in responses
- Multiple test endpoints

## Endpoints

- `GET /ping` - Health check
- `GET /users/:id` - Get user by ID
- `GET /panic` - Trigger panic (recovery test)
- `GET /error` - Return client error

## Running

```bash
export SERVICE_NAME=gin-service
export PORT=8080
export METRICS_PORT=9090
export OTEL_ENDPOINT=localhost:4318
go run main.go
```

## Testing

```bash
# Normal request
curl http://localhost:8080/ping

# User request
curl http://localhost:8080/users/123

# Panic test (will be recovered)
curl http://localhost:8080/panic

# Error response
curl http://localhost:8080/error
```
