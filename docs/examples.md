# Examples

This repo includes several example services in the `examples/` folder:

- `examples/gin-example`
- `examples/gin-service`
- `examples/grpc-service`
- `examples/simple-service`

Each example contains a `README.md` with run instructions. For local quick-run:

```bash
cd examples/gin-service
go run main.go
```

---

## Detailed examples

### Gin Example (`examples/gin-example`)

Purpose: demonstrates middleware with route skipping, panic recovery, tracing and Prometheus
metrics.

Run:

```bash
export SERVICE_NAME=gin-example
export LOG_LEVEL=info
export OTEL_ENDPOINT=localhost:4318
export METRICS_PORT=9090
export PORT=8080
cd examples/gin-example
go run main.go
```

For local dev with a non-TLS collector set:

```bash
export OTEL_INSECURE=true
export METRICS_INSECURE=true   # if using push exporters against a non-TLS collector
go run main.go
```

Key endpoints:

- `GET /ping` — tracked
- `GET /users/:id` — tracked
- `GET /health`, `/metrics`, `/status` — skipped by default
- `GET /error` — triggers panic/recovery

Integration: start `e2e` stack to view Jaeger/Prometheus UIs.

### Gin Service (`examples/gin-service`)

Purpose: simple Gin service showcasing endpoints and observability middleware.

Run:

```bash
export SERVICE_NAME=gin-service
export PORT=8080
export METRICS_PORT=9090
export OTEL_ENDPOINT=localhost:4318
go run examples/gin-service/main.go
```

Local-dev tip:

```bash
export OTEL_INSECURE=true
go run examples/gin-service/main.go
```

Test:

```bash
curl http://localhost:8080/ping
curl http://localhost:8080/users/123
curl http://localhost:8080/panic
curl http://localhost:8080/error
```

### gRPC Service (`examples/grpc-service`)

Purpose: demonstrates unary and streaming interceptors, panic recovery, and grpcurl testing.

Run:

```bash
export SERVICE_NAME=grpc-service
export OTEL_ENDPOINT=localhost:4318
export METRICS_PORT=9090
cd examples/grpc-service
go run main.go
```

If your OTLP collector is running without TLS locally:

```bash
export OTEL_INSECURE=true
export METRICS_INSECURE=true
go run main.go
```

Test with `grpcurl`:

```bash
grpcurl -plaintext -d '{"name":"World"}' localhost:50051 hello.HelloService/SayHello
grpcurl -plaintext -d '{"name":"Stream"}' localhost:50051 hello.HelloService/SayHelloStream
```

### Simple Service (`examples/simple-service`)

Purpose: minimal service showing tracing and metrics with a `/ping` endpoint.

Run:

```bash
export SERVICE_NAME=simple-service
export PORT=8080
export METRICS_PORT=9090
export OTEL_ENDPOINT=localhost:4318
go run examples/simple-service/main.go
```

Local dev:

```bash
export OTEL_INSECURE=true
go run examples/simple-service/main.go
```

Test:

```bash
curl http://localhost:8080/ping
```

---

Each example includes its README for more details. Use the `e2e` stack in `e2e/` to view Jaeger and
Prometheus dashboards when running examples with OTEL configured.
