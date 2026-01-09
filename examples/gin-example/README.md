# Gin Middleware Example

This example demonstrates how to use the Ecoma Go Observability library with Gin framework.

## Features Demonstrated

- ✅ HTTP request logging with trace context
- ✅ Automatic panic recovery with structured error responses
- ✅ Trace ID propagation in responses
- ✅ OpenTelemetry tracing integration
- ✅ Prometheus metrics collection

## Running the Example

### 1. Set environment variables

```bash
export SERVICE_NAME=gin-example
export LOG_LEVEL=info
export OTEL_ENDPOINT=localhost:4318
export METRICS_PORT=9090
export PORT=8080
```

### 2. Run the service

```bash
cd examples/gin-example
go run main.go
```

### 3. Test the endpoints

**Normal request:**

```bash
curl http://localhost:8080/ping
```

**User endpoint with tracing:**

```bash
curl http://localhost:8080/users/123
```

**Trigger panic (recovery test):**

```bash
curl http://localhost:8080/error
```

**Client error:**

```bash
curl http://localhost:8080/not-found
```

## Expected Logs

### Success Request (200)

```json
{
  "level": "info",
  "timestamp": "2026-01-09T05:30:00.123Z",
  "msg": "HTTP Request",
  "service": "gin-example",
  "status": 200,
  "method": "GET",
  "path": "/ping",
  "latency_ms": 10,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

### Panic Recovery (500)

```json
{
  "level": "error",
  "timestamp": "2026-01-09T05:30:00.456Z",
  "msg": "Panic recovered",
  "service": "gin-example",
  "error": "something went wrong!",
  "trace_id": "a1b2c3d4e5f6...",
  "path": "/error",
  "method": "GET",
  "stack": "goroutine 42 [running]:\n..."
}
```

**Client receives:**

```json
{
  "error": "Internal Server Error",
  "message": "An unexpected error occurred. Please try again later.",
  "trace_id": "a1b2c3d4e5f6...",
  "path": "/error"
}
```

## Integration with Full Stack

To see traces in Jaeger and metrics in Prometheus:

1. Start the E2E infrastructure:

```bash
cd ../../e2e
docker-compose up -d
```

2. Run this example with proper OTEL endpoint:

```bash
export OTEL_ENDPOINT=localhost:4318
go run main.go
```

3. Access the UIs:

- Jaeger: http://localhost:16687
- Prometheus: http://localhost:9099
- Metrics endpoint: http://localhost:9090/metrics

## Code Structure

```go
// Apply middleware to Gin router
for _, mw := range observability.GinMiddleware(logger) {
    router.Use(mw)
}

// Or apply individually:
// router.Use(observability.GinRecovery(logger))
// router.Use(observability.GinLogger(logger))
```

## Middleware Order

The middleware is applied in this order:

1. **GinRecovery** - Catches panics first
2. **GinLogger** - Logs all requests (including recovered ones)

This ensures that even panic-recovered requests are properly logged.
