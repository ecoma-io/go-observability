<!-- markdownlint-disable -->
<div align="center">

[![Open in DevContainer](https://img.shields.io/badge/Open-DevContainer-blue.svg)](https://vscode.dev/redirect?url=vscode://ms-vscode-remote.remote-containers/cloneInVolume?url=https://github.com/ecoma-io/services)

</div>
<div align="center">
  <a href="https://ecoma.io">
    <img src="https://github.com/ecoma-io/.github/blob/main/.github/logo.png?raw=true" alt="Logo" height="60">
  </a>
</div>
<!-- markdownlint-restore -->

# Go Observability

A standardized library for Golang microservices at **Ecoma-io**, providing built-in
**Configuration**, **Structured Logging (Zap)**, and **Observability (OpenTelemetry)**.

## 🚀 Key Features

- **Unified Config**: Load configuration from multiple sources (.env, Environment Variables,
  LDFlags).
- **Structured Logging**: Robust JSON logging with Zap, automatically attaching service metadata.
- **Distributed Tracing**: OpenTelemetry Tracing integration via OTLP/HTTP.
- **Metrics**: Collect and expose system metrics using Prometheus (Pull model).
- **Build-time Metadata**: Support for injecting Version and Build Time from CI/CD.
- **Gin Middleware**: Ready-to-use middleware for Gin framework with logging, tracing, and panic
  recovery.
- **gRPC Interceptors**: Unary and streaming interceptors for gRPC servers with full observability.

---

## 📦 Installation

```bash
go get github.com/ecoma-io/go-observability
```

## 🛠 Usage

### 1. Initialize configuration and Logger

The library is designed to be easily embedded into each service's configuration struct.

```go
package main

import (
    "context"
    "github.com/ecoma-io/go-observability"
)

func main() {
    // Define the service config struct by embedding BaseConfig
    type Config struct {
        observability.BaseConfig
        DatabaseURL string `env:"DATABASE_URL"`
    }

    var cfg Config
    
    // 1. Load configuration (Order: LDFlags > .env > Environment Variables)
    if err := observability.LoadCfg(&cfg); err != nil {
        panic(err)
    }

    // 2. Initialize Logger
    logger := observability.NewLogger(&cfg.BaseConfig)
    defer logger.Sync()

    logger.Info("Service started", "version", cfg.Version, "env", "production")

    // 3. Initialize OpenTelemetry (Tracing & Metrics)
    shutdown, err := observability.InitOtel(cfg.BaseConfig)
    if err != nil {
        logger.Fatal("Failed to init Otel", "error", err)
    }
    defer shutdown(context.Background())
}
```

### 2. Use Tracing & Metrics in your code

```go
// Get a Tracer to create Spans for tasks
tracer := observability.GetTracer("inventory-handler")
ctx, span := tracer.Start(ctx, "update-stock")
defer span.End()

// Get a Meter to record measurements (Business Metrics)
meter := observability.GetMeter("inventory-service")
counter, _ := meter.Int64Counter("stock_updates_total")
counter.Add(ctx, 1)
```

### 3. Gin Middleware Integration

The library provides ready-to-use middleware for Gin framework with automatic tracing, logging, and
panic recovery:

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/ecoma-io/go-observability"
)

func main() {
    // ... initialize config, logger, otel ...

    router := gin.Default()

    // Apply all observability middleware (recommended)
    for _, mw := range observability.GinMiddleware(logger, cfg.ServiceName) {
        router.Use(mw)
    }

    // Or apply individually:
    // router.Use(observability.GinTracing(cfg.ServiceName)) // Create tracing spans
    // router.Use(observability.GinRecovery(logger))          // Panic recovery
    // router.Use(observability.GinLogger(logger))            // Request logging

    router.GET("/ping", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "pong"})
    })

    router.Run(":8080")
}
```

**Features:**

- ✅ Automatic tracing span creation for each HTTP request
- ✅ W3C Trace Context propagation (extract from headers, inject into response)
- ✅ Request/response logging with trace context
- ✅ Panic recovery with structured error responses
- ✅ Trace ID in response headers (X-Trace-ID) and error responses
- ✅ Status-based log levels (info/warn/error)
- ✅ Route skipping capabilities for health checks and metrics endpoints

#### Route Skipping (Skip Health Checks & Metrics)

The Gin middleware supports skipping observability tracking for certain routes, such as `/health` or
`/metrics` endpoints. This reduces noise in logs and traces.

**Option 1: Using ExcludedPaths (list of paths)**

```go
middlewareCfg := &observability.ObservabilityMiddlewareConfig{
    ExcludedPaths: []string{"/health", "/metrics", "/status"},
}

// Apply middleware with skip configuration
for _, mw := range observability.GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareCfg) {
    router.Use(mw)
}
```

**Option 2: Using SkipRoute (custom predicate function)**

```go
import "strings"

middlewareCfg := &observability.ObservabilityMiddlewareConfig{
    SkipRoute: func(path string) bool {
        // Skip paths that start with /health or /metrics
        return strings.HasPrefix(path, "/health") ||
               strings.HasPrefix(path, "/metrics")
    },
}

for _, mw := range observability.GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareCfg) {
    router.Use(mw)
}
```

**Behavior for Skipped Routes:**

- 🚫 No span is created for tracing
- 🚫 No request is logged
- 🚫 No metrics are recorded

This ensures that frequently accessed health check and metrics endpoints don't clutter your
observability data.

**Example Log Output:**

```json
{
  "level": "info",
  "timestamp": "2026-01-09T05:30:00.123Z",
  "msg": "HTTP Request",
  "service": "my-service",
  "status": 200,
  "method": "GET",
  "path": "/api/users",
  "latency_ms": 15,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

See [examples/gin-example](examples/gin-example) for a complete working example.

### 4. gRPC Interceptors Integration

The library provides ready-to-use interceptors for gRPC servers with automatic logging, tracing, and
panic recovery:

```go
package main

import (
    "net"
    "github.com/ecoma-io/go-observability"
    "google.golang.org/grpc"
)

func main() {
    // ... initialize config, logger, otel ...

    // Create gRPC server with observability interceptors
    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            observability.GrpcUnaryInterceptors(logger)...,
        ),
        grpc.ChainStreamInterceptor(
            observability.GrpcStreamInterceptors(logger)...,
        ),
    )

    // Or apply individually:
    // grpc.NewServer(
    //     grpc.UnaryInterceptor(observability.GrpcUnaryRecoveryInterceptor(logger)),
    //     grpc.UnaryInterceptor(observability.GrpcUnaryServerInterceptor(logger)),
    //     grpc.StreamInterceptor(observability.GrpcStreamRecoveryInterceptor(logger)),
    //     grpc.StreamInterceptor(observability.GrpcStreamServerInterceptor(logger)),
    // )

    // Register your gRPC services
    // pb.RegisterYourServiceServer(server, &yourServiceImpl{})

    lis, _ := net.Listen("tcp", ":50051")
    server.Serve(lis)
}
```

**Features:**

- ✅ Automatic request/response logging for unary and streaming RPCs
- ✅ Panic recovery with gRPC status errors
- ✅ Trace ID propagation in response metadata
- ✅ Status-based log levels (info/warn/error)
- ✅ Support for both unary and streaming interceptors

**Example Log Output (Unary RPC):**

```json
{
  "level": "info",
  "timestamp": "2026-01-09T08:30:00.123Z",
  "msg": "gRPC Request",
  "service": "my-grpc-service",
  "method": "/pb.UserService/GetUser",
  "grpc_code": "OK",
  "latency_ms": 25,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

**Example Log Output (Stream RPC):**

```json
{
  "level": "info",
  "timestamp": "2026-01-09T08:30:00.456Z",
  "msg": "gRPC Stream Request",
  "service": "my-grpc-service",
  "method": "/pb.ChatService/StreamMessages",
  "grpc_code": "OK",
  "latency_ms": 5000,
  "is_client_stream": true,
  "is_server_stream": true,
  "trace_id": "a1b2c3d4e5f6...",
  "span_id": "1234567890ab..."
}
```

## 🏗 Build with LDFlags (Recommended)

To make full use of version management, use `-ldflags` to inject information into the binary at
build time:

```bash
SERVICE_NAME="order-service"
VERSION="v1.2.3"
BUILD_TIME=$(date +%FT%T%z)
MODULE_PATH="github.com/ecoma-io/go-observability"

go build -ldflags "-X '$MODULE_PATH.ServiceName=$SERVICE_NAME' \
                   -X '$MODULE_PATH.Version=$VERSION' \
                   -X '$MODULE_PATH.BuildTime=$BUILD_TIME'" \
         -o main .
```

## ⚙️ Environment Variables (Configuration)

| Variable                   | Default          | Description                                           |
| :------------------------- | :--------------- | :---------------------------------------------------- |
| `SERVICE_NAME`             | **(Required)**   | Service name (Used as `service.name` in Otel/Log)     |
| `LOG_LEVEL`                | `info`           | Log level (`debug`, `info`, `warn`, `error`)          |
| `OTEL_ENDPOINT`            | `localhost:4318` | OTLP Collector address over HTTP (for Tracing)        |
| `METRICS_MODE`             | `pull`           | Metrics collection mode: `pull`, `push`, or `hybrid`  |
| `METRICS_PORT`             | `9090`           | Port to run the Metrics server (Prometheus format)    |
| `METRICS_PATH`             | `/metrics`       | HTTP endpoint path for Prometheus metrics (pull mode) |
| `METRICS_PUSH_ENDPOINT`    | **(Optional)**   | OTLP metrics collector endpoint (for push mode)       |
| `METRICS_PUSH_INTERVAL`    | `30`             | Metrics push interval in seconds (push mode)          |
| `OTEL_TRACING_SAMPLE_RATE` | `1.0`            | Tracing sample rate (0.0 to 1.0)                      |

### 📊 Metrics Modes

The library supports three metrics collection modes:

#### 1. **Pull Mode (Default)** - Prometheus Scraping

In pull mode, the service exposes a Prometheus metrics endpoint that Prometheus scrapes at regular
intervals.

**Configuration:**

```bash
METRICS_MODE=pull
METRICS_PORT=9090
METRICS_PATH=/metrics
```

**Example Setup:**

```yaml
# docker-compose.yml
services:
  my-service:
    environment:
      METRICS_MODE: pull
      METRICS_PORT: 9090

  prometheus:
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    command: --config.file=/etc/prometheus/prometheus.yml
```

**prometheus.yml:**

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: "my-service"
    static_configs:
      - targets: ["my-service:9090"]
```

**Benefits:**

- ✅ Simple deployment (no external metrics pipeline needed)
- ✅ Services are in control of metric emission
- ✅ Lower latency for metric queries
- ✅ Compatible with Prometheus ecosystem

#### 2. **Push Mode** - OTLP Push to Collector

In push mode, the service actively pushes metrics to an OTLP metrics collector at regular intervals.

**Configuration:**

```bash
METRICS_MODE=push
METRICS_PUSH_ENDPOINT=localhost:4318
METRICS_PUSH_INTERVAL=30
```

**Example Setup:**

```yaml
# docker-compose.yml
services:
  my-service:
    environment:
      METRICS_MODE: push
      METRICS_PUSH_ENDPOINT: otel-collector:4318
      METRICS_PUSH_INTERVAL: 30

  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./otel-config.yaml:/etc/otel/config.yaml
```

**Benefits:**

- ✅ Suitable for ephemeral/serverless workloads
- ✅ Centralized metrics collection
- ✅ Integration with OTLP ecosystem (Datadog, New Relic, etc.)
- ✅ No need for Prometheus scraping configuration

#### 3. **Hybrid Mode** - Pull + Push

In hybrid mode, metrics are both exposed via Prometheus endpoint AND pushed to an OTLP collector.

**Configuration:**

```bash
METRICS_MODE=hybrid
METRICS_PORT=9090
METRICS_PATH=/metrics
METRICS_PUSH_ENDPOINT=localhost:4318
METRICS_PUSH_INTERVAL=30
```

**Benefits:**

- ✅ Flexibility to use both Prometheus and OTLP collectors
- ✅ Fallback: if push fails, pull still works
- ✅ Redundancy in metrics collection
- ✅ Gradual migration path from pull to push

### Native OTLP Metric Export

The library provides native support for OTLP (OpenTelemetry Line Protocol) metric export via both
**HTTP** and **gRPC** protocols, enabling seamless integration with OpenTelemetry Collectors and
modern observability platforms.

#### Protocol Selection

Configure the protocol used for OTLP metric export using the `METRICS_PROTOCOL` environment
variable:

```bash
# HTTP Protocol (default, recommended for compatibility)
METRICS_PROTOCOL=http
METRICS_PUSH_ENDPOINT=localhost:4318

# gRPC Protocol (lower latency, more efficient)
METRICS_PROTOCOL=grpc
METRICS_PUSH_ENDPOINT=localhost:4317
```

**Protocol Comparison:**

| Feature           | HTTP   | gRPC                 |
| ----------------- | ------ | -------------------- |
| Default Port      | 4318   | 4317                 |
| Overhead          | Higher | Lower                |
| Latency           | Higher | Lower                |
| Compatibility     | Wider  | OTLP native          |
| Firewall Friendly | Yes    | No (custom protocol) |

#### HTTP Protocol Configuration

**Example with HTTP (Default):**

```bash
# Environment variables
METRICS_MODE=push
METRICS_PROTOCOL=http
METRICS_PUSH_ENDPOINT=otel-collector:4318
METRICS_PUSH_INTERVAL=30
```

**Code Example:**

```go
package main

import (
    "context"
    "github.com/ecoma-io/go-observability"
)

func main() {
    cfg := observability.BaseConfig{
        ServiceName:           "my-service",
        MetricsMode:           "push",
        MetricsProtocol:       "http",  // HTTP Protocol
        MetricsPushEndpoint:   "otel-collector:4318",
        MetricsPushInterval:   30,
    }

    shutdown, err := observability.InitOtel(cfg)
    if err != nil {
        panic(err)
    }
    defer shutdown(context.Background())

    // Your service code...
}
```

#### gRPC Protocol Configuration

**Example with gRPC:**

```bash
# Environment variables
METRICS_MODE=push
METRICS_PROTOCOL=grpc
METRICS_PUSH_ENDPOINT=otel-collector:4317
METRICS_PUSH_INTERVAL=30
```

**Code Example:**

```go
package main

import (
    "context"
    "github.com/ecoma-io/go-observability"
)

func main() {
    cfg := observability.BaseConfig{
        ServiceName:           "my-service",
        MetricsMode:           "push",
        MetricsProtocol:       "grpc",  // gRPC Protocol
        MetricsPushEndpoint:   "otel-collector:4317",
        MetricsPushInterval:   30,
    }

    shutdown, err := observability.InitOtel(cfg)
    if err != nil {
        panic(err)
    }
    defer shutdown(context.Background())

    // Your service code...
}
```

#### OpenTelemetry Collector Configuration

**docker-compose.yml:**

```yaml
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    ports:
      - "4317:4317" # OTLP gRPC receiver
      - "4318:4318" # OTLP HTTP receiver
    volumes:
      - ./otel-config.yaml:/etc/otel/config.yaml
    command: ["--config=/etc/otel/config.yaml"]
```

**otel-config.yaml:**

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  # Export to Prometheus
  prometheus:
    endpoint: "0.0.0.0:8889"

  # Export to Datadog (example)
  datadog:
    api:
      key: "${DD_API_KEY}"
      site: "datadoghq.com"

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: []
      exporters: [prometheus, datadog]
```

#### ForceFlush for Short-Lived Jobs

The library implements **ForceFlush** in the shutdown sequence to ensure all metrics are sent before
the service terminates. This is especially important for batch jobs and Lambda/Serverless functions.

**Shutdown Process:**

```go
shutdown, err := observability.InitOtel(cfg)
if err != nil {
    panic(err)
}

// Your service code...

// On graceful shutdown, ForceFlush is called automatically
ctx := context.Background()
defer shutdown(ctx)  // Calls ForceFlush on MeterProvider and TracerProvider
```

**What ForceFlush does:**

1. ✅ Flushes all in-flight metrics to the OTLP collector
2. ✅ Flushes all in-flight traces
3. ✅ Ensures no data loss on service termination
4. ✅ Waits with a reasonable timeout (configured context)

**Example with Timeout:**

```go
// Graceful shutdown with 5 second timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := shutdown(shutdownCtx); err != nil {
    logger.Error("Shutdown error", "error", err)
}
```

#### Backward Compatibility

The library maintains **full backward compatibility**:

- Existing configurations continue to work without changes
- Default protocol is HTTP for maximum compatibility
- Pull mode and Prometheus users are unaffected
- All existing metrics modes (pull, push, hybrid) continue to work

### Helper Methods

The `BaseConfig` struct provides helper methods to check metrics mode:

```go
cfg := BaseConfig{
    MetricsMode: "push",
}

if cfg.IsPull() {    // Check if pull is enabled (pull or hybrid)
    // Setup pull metrics endpoint
}

if cfg.IsPush() {    // Check if push is enabled (push or hybrid)
    // Setup push exporter
}

if cfg.IsHybrid() {  // Check if hybrid mode (both pull and push)
    // Setup both
}
```

## 🧪 Testing

### Unit Tests

Run unit tests for the library:

```bash
go test -v ./...
```

### End-to-End (E2E) Tests

The library includes a comprehensive E2E test suite that verifies the complete observability stack
in a real-world environment using Docker.

**What is tested:**

- ✅ **Distributed Tracing**: Traces are sent to OpenTelemetry Collector and viewable in Jaeger
- ✅ **Metrics Collection**: Prometheus successfully scrapes metrics from the service
- ✅ **Structured Logging**: JSON logs with trace context (trace_id, span_id)
- ✅ **Integration**: Full stack integration (Service → OTEL Collector → Jaeger/Prometheus)

**Prerequisites:**

- Docker and Docker Compose
- Go 1.25+
- Available ports: 8081, 9092, 9099, 14318, 16687

**Run E2E tests:**

```bash
cd e2e
./run-e2e.sh
```

The test suite will:

1. Start infrastructure (Jaeger, Prometheus, OTEL Collector) using Docker Compose
2. Build and run the example service
3. Generate test traffic (5 HTTP requests)
4. Verify traces appear in Jaeger
5. Verify metrics are scraped by Prometheus
6. Clean up all resources automatically

**Architecture:**

```
┌─────────────────┐
│ Simple Service  │ (Port 8081)
│   /ping         │
└────────┬────────┘
         │ OTLP/HTTP
         ▼
┌─────────────────┐
│ OTEL Collector  │ (Port 14318)
└────┬────────┬───┘
     │        │
     │        └──────────────┐
     │                       │
     ▼                       ▼
┌─────────┐          ┌─────────────┐
│ Jaeger  │          │ Prometheus  │
│ (16687) │          │   (9099)    │
└─────────┘          └─────────────┘
```

For more details about the E2E test implementation, see [E2E.md](E2E.md).
