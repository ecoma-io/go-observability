# Simple Service Example

A basic HTTP service demonstrating the core observability features.

## Features

- HTTP server with `/ping` endpoint
- OpenTelemetry tracing
- Prometheus metrics
- Structured logging with trace context

## Running

```bash
export SERVICE_NAME=simple-service
export PORT=8080
export METRICS_PORT=9090
export OTEL_ENDPOINT=localhost:4318
go run main.go
```

Local dev (non-TLS collector):

```bash
export OTEL_INSECURE=true
go run main.go
```

## Testing

```bash
curl http://localhost:8080/ping
```
