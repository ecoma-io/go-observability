# Gin Middleware Example

This example demonstrates how to use the Ecoma Go Observability library with Gin framework,
including advanced features like route skipping for health checks and metrics.

## Features Demonstrated

- ✅ HTTP request logging with trace context
- ✅ Automatic panic recovery with structured error responses
- ✅ Trace ID propagation in responses
- ✅ OpenTelemetry tracing integration
- ✅ Prometheus metrics collection
- ✅ Route skipping for health checks and metrics endpoints

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

**Normal request (tracked):**

```bash
curl http://localhost:8080/ping
```

**User endpoint with tracing (tracked):**

```bash
curl http://localhost:8080/users/123
```

**Health check (NOT tracked - skipped):**

```bash
curl http://localhost:8080/health
```

**Metrics endpoint (NOT tracked - skipped):**

```bash
curl http://localhost:8080/metrics
```

**Status endpoint (NOT tracked - skipped):**

```bash
curl http://localhost:8080/status
```

**Trigger panic (recovery test - tracked):**

```bash
curl http://localhost:8080/error
```

**Client error (tracked):**

```bash
curl http://localhost:8080/not-found
```

## Expected Logs

### Success Request (200) - TRACKED

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

### Health Check (200) - SKIPPED

❌ **No log entry** - Health checks are excluded from observability tracking

```bash
# No JSON log output for this request
```

### Panic Recovery (500) - TRACKED

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

## Route Skipping Configuration

This example uses a **custom predicate function** to skip observability for health and metrics
endpoints:

```go
middlewareConfig := &observability.ObservabilityMiddlewareConfig{
    SkipRoute: func(path string) bool {
        // Skip health checks and metrics endpoints
        return strings.HasPrefix(path, "/health") ||
               strings.HasPrefix(path, "/metrics") ||
               path == "/status"
    },
}

// Apply middleware with skip configuration
for _, mw := range observability.GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareConfig) {
    router.Use(mw)
}
```

### Alternative: Using ExcludedPaths

If you prefer a simple list of paths:

```go
middlewareConfig := &observability.ObservabilityMiddlewareConfig{
    ExcludedPaths: []string{"/health", "/metrics", "/status"},
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
// Create middleware config to skip health and metrics endpoints
middlewareConfig := &observability.ObservabilityMiddlewareConfig{
    SkipRoute: func(path string) bool {
        return strings.HasPrefix(path, "/health") ||
               strings.HasPrefix(path, "/metrics") ||
               path == "/status"
    },
}

// Apply middleware to Gin router with skip configuration
for _, mw := range observability.GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareConfig) {
    router.Use(mw)
}

// Or apply middleware without skip (tracks all routes):
// for _, mw := range observability.GinMiddleware(logger, cfg.ServiceName) {
//     router.Use(mw)
// }
```

## Middleware Order

The middleware is applied in this order:

1. **GinTracing** - Creates tracing spans (or skips if route is excluded)
2. **GinRecovery** - Catches panics first (respects skip configuration)
3. **GinLogger** - Logs all requests (or skips if route is excluded)

This ensures that even panic-recovered requests are properly logged, unless explicitly skipped.

### Benefits of Route Skipping

- 📉 **Reduced Noise**: Health checks won't clutter your traces and logs
- ⚡ **Better Performance**: Skipped routes don't incur observability overhead
- 🎯 **Cleaner Signals**: Metrics and traces focus on actual business logic
- 🔍 **Easier Debugging**: Less noise makes it easier to find important events
