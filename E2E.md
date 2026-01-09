# End-to-End (E2E) Testing Guide

This document describes the E2E testing strategy for the **go-observability** library, ensuring that
all observability components (logging, tracing, and metrics) work correctly in a real-world
environment.

## Overview

The E2E test suite validates the complete observability stack by:

- Building and running all services in Docker containers for consistent environment
- Using Docker Compose to orchestrate infrastructure (Jaeger, Prometheus, OpenTelemetry Collector)
- Running a sample service that uses the observability library with build-time metadata injection
- Generating test traffic
- Verifying that traces, metrics, logs, and build metadata are correctly collected and exported

## Test Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   E2E Test Suite (Bash Script)               │
│                 • Builds Docker images with LDFlags          │
│                 • Orchestrates all services                   │
│                 • Generates traffic and verifies results     │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
│   Jaeger     │    │ OTEL Collector   │    │ Prometheus   │
│  (Traces)    │◄───│   (Gateway)      │───►│  (Metrics)   │
└──────────────┘    └──────────────────┘    └──────────────┘
                             ▲
                             │ OTLP/HTTP  
                             │
                    ┌────────┴────────┐
                    │ Simple Service  │
                    │ (Docker)        │
                    │ Built with      │
                    │ LDFlags         │
                    └─────────────────┘
```

## Components

### 1. Infrastructure Services (Docker Compose)

| Service            | Image                                  | Port(s)           | Purpose                                  |
| :----------------- | :------------------------------------- | :---------------- | :--------------------------------------- |
| **simple-service** | Built from Dockerfile (multi-stage)    | 8081, 9092        | Basic HTTP service with observability    |
| **gin-service**    | Built from Dockerfile (multi-stage)    | 8082, 9093        | Gin framework with middleware            |
| **grpc-service**   | Built from Dockerfile (multi-stage)    | 50051, 8083, 9094 | gRPC service with interceptors           |
| **otel-collector** | `otel/opentelemetry-collector-contrib` | 4318, 4317        | Receives OTLP data from services         |
| **jaeger**         | `jaegertracing/all-in-one`             | 16686, 14268      | Stores and visualizes distributed traces |
| **prometheus**     | `prom/prometheus`                      | 9090              | Scrapes and stores metrics               |

### 2. Test Applications

#### simple-service

A minimal Go HTTP service demonstrating basic observability:

- **Endpoint**: `GET /ping` returns "pong"
- **Features**:
  - Creates a trace span for each request
  - Increments a counter metric (`request_count_total`)
  - Logs request details with trace context
  - Introduces random latency (0-100ms)

#### gin-service

A Gin framework service demonstrating HTTP middleware:

- **Endpoints**:
  - `GET /ping` - Basic health check
  - `GET /users/:id` - User retrieval with params
  - `GET /panic` - Tests panic recovery middleware
  - `GET /error` - Tests error handling
- **Features**:
  - Automatic request/response logging via `GinLogger` middleware
  - Panic recovery with structured errors via `GinRecovery` middleware
  - Trace ID propagation in error responses
  - Status-based log levels

#### grpc-service

A gRPC service demonstrating gRPC interceptors:

- **Service**: `HelloService`
  - `SayHello` (Unary RPC) - Simple greeting
  - `SayHelloStream` (Server Streaming RPC) - Multiple greetings
- **Features**:
  - Automatic request/response logging via `GrpcUnaryServerInterceptor` and
    `GrpcStreamServerInterceptor`
  - Panic recovery with gRPC status errors via recovery interceptors
  - Trace ID propagation in metadata
  - Health check endpoint for readiness probes
  - gRPC reflection enabled for testing with grpcurl

All services are built with:

- Multi-stage Docker build with LDFlags injection
- Build arguments: `VERSION`, `BUILD_TIME`, `SERVICE_NAME`

### 3. Test Script

**run-e2e.sh** orchestrates the entire test flow:

1. Sets build-time environment variables (BUILD_TIME)
2. Builds Docker images with LDFlags using `docker compose build`
3. Starts all services with `docker compose up -d --build`
4. Waits for services to be ready (health checks)
5. Sends HTTP requests to simple-service and gin-service
6. Sends gRPC requests to grpc-service (if grpcurl is available)
7. Tests panic recovery and error handling
8. Verifies build metadata in service logs
9. Verifies traces in Jaeger for all services
10. Verifies metrics in Prometheus for all services
11. Cleans up all resources

**Key Features:**

- Uses absolute paths - can be run from any directory
- POSIX-compliant shell script (works with sh, bash, zsh)
- Automatic cleanup on exit or interrupt (Ctrl+C)
- Detailed verification with clear success/failure messages

## Running E2E Tests

### Prerequisites

- **Docker** and **Docker Compose** installed
- **Go 1.21+** installed
- Ports available: 8081-8083, 9092-9094, 9099, 14318, 16687, 50051
- Optional: **grpcurl** for gRPC testing (tests will run without it)

### Execute Tests

From the project root or e2e directory:

```bash
# From project root
./e2e/run-e2e.sh

# Or from e2e directory
cd e2e
./run-e2e.sh
```

**Note:** The script uses absolute path resolution, so it works from any directory.

### Expected Output

```
Starting E2E Tests...
Project root: /workspaces/backend
Script directory: /workspaces/backend/e2e
[1/3] Building and Starting Docker Infrastructure...
  • Building Docker images with LDFlags
  • Starting all services
Waiting for simple-service... ready (3s)
Waiting for gin-service... ready (3s)
Waiting for grpc-service... ready (4s)
Waiting for jaeger... ready (2s)
[2/3] Generating Load...
Testing simple-service...
Testing gin-service...
Testing panic recovery...
SUCCESS: Panic recovery works!
  + trace_id included in response
Testing grpc-service...
SUCCESS: gRPC requests sent
[3/4] Verifying Observability...
Waiting for traces to appear... traces ready
[4/4] Verifying Build Metadata (LDFlags)...
SUCCESS: Version metadata found in logs!
Checking Jaeger for simple-service traces...
SUCCESS: simple-service traces found in Jaeger!
Checking Jaeger for gin-service traces...
SUCCESS: gin-service traces found in Jaeger!
Checking Jaeger for grpc-service traces...
SUCCESS: grpc-service traces found in Jaeger!
Checking Prometheus for simple-service metrics...
SUCCESS: simple-service metrics found in Prometheus!
Checking Prometheus for gin-service metrics...
SUCCESS: gin-service metrics available (not yet in Prometheus, but endpoint works)
Checking Prometheus for grpc-service metrics...
SUCCESS: grpc-service metrics available (not yet in Prometheus, but endpoint works)
All E2E Tests Passed!
```

## Verification Details

### Build Metadata Verification (LDFlags)

The test verifies that build-time metadata is correctly injected via LDFlags:

```bash
docker logs e2e-simple-service-1 | grep version
```

**Success criteria**: Service logs contain version `v1.0.0-e2e`

**Example log output:**

```json
{
  "level": "info",
  "msg": "Starting simple-service",
  "service": "simple-service",
  "version": "v1.0.0-e2e"
}
```

This validates the LDFlags feature described in the README.md.

### Traces Verification

The test queries the Jaeger API to confirm traces exist for all services:

```bash
GET http://localhost:16687/api/traces?service=simple-service
GET http://localhost:16687/api/traces?service=gin-service
GET http://localhost:16687/api/traces?service=grpc-service
```

**Success criteria**: Response contains `traceID` field

**What's tested:**

- simple-service: Basic HTTP request tracing
- gin-service: Gin middleware automatic tracing with trace_id in error responses
- grpc-service: Unary and streaming RPC tracing with interceptors

### Metrics Verification

The test queries the Prometheus API for counter metrics from all services:

```bash
GET http://localhost:9099/api/v1/query?query=request_count_total
GET http://localhost:9099/api/v1/query?query=gin_request_count_total
GET http://localhost:9099/api/v1/query?query=grpc_request_count_total
```

**Success criteria**:

- Response status is "success"
- Metrics contain appropriate service labels
- Counter values are present

**Fallback**: If Prometheus hasn't scraped yet, the test checks metrics endpoints directly:

```bash
curl http://localhost:9092/metrics  # simple-service
curl http://localhost:9093/metrics  # gin-service
curl http://localhost:9094/metrics  # grpc-service
```

### Logs Verification

Logs are output to stdout in JSON format with structured fields:

```json
{
  "level": "info",
  "timestamp": "2026-01-07T18:09:28.413Z",
  "caller": "backend/logger.go:46",
  "msg": "Ping received",
  "service": "simple-service",
  "version": "dev",
  "trace_id": "f64b9e3141d6692458b7516b3fa334c2",
  "span_id": "2011d2c67f941dfc",
  "latency_ms": 95
}
```

## Test Configuration

### Environment Variables

Each test service receives environment variables from docker-compose.yml:

**simple-service:** | Variable | Value | Description | | :-------------- |
:--------------------------- | :------------------------------------ | | `SERVICE_NAME` |
`simple-service` | Service identifier | | `PORT` | `8080` (mapped to host 8081) | HTTP server port |
| `METRICS_PORT` | `9090` (mapped to host 9092) | Prometheus metrics endpoint port | |
`OTEL_ENDPOINT` | `otel-collector:4318` | OpenTelemetry Collector HTTP endpoint | | `LOG_LEVEL` |
`info` | Logging level |

**gin-service:** | Variable | Value | Description | | :-------------- | :---------------------------
| :------------------------------------ | | `SERVICE_NAME` | `gin-service` | Service identifier | |
`PORT` | `8080` (mapped to host 8082) | HTTP server port | | `METRICS_PORT` | `9090` (mapped to
host 9093) | Prometheus metrics endpoint port | | `OTEL_ENDPOINT` | `otel-collector:4318` |
OpenTelemetry Collector HTTP endpoint | | `LOG_LEVEL` | `info` | Logging level |

**grpc-service:** | Variable | Value | Description | | :-------------- |
:--------------------------- | :------------------------------------ | | `SERVICE_NAME` |
`grpc-service` | Service identifier | | `PORT` | `50051` (mapped to host 50051) | gRPC server port |
| `HEALTH_PORT` | `8080` (mapped to host 8083) | HTTP health check port | | `METRICS_PORT` | `9090`
(mapped to host 9094) | Prometheus metrics endpoint port | | `OTEL_ENDPOINT` | `otel-collector:4318`
| OpenTelemetry Collector HTTP endpoint | | `LOG_LEVEL` | `info` | Logging level |

### Build Arguments (Docker)

These are set at image build time via docker-compose.yml for each service:

| Argument       | simple-service       | gin-service       | grpc-service       | Purpose                            |
| :------------- | :------------------- | :---------------- | :----------------- | :--------------------------------- |
| `VERSION`      | `v1.0.0-e2e`         | `v1.0.0-e2e-gin`  | `v1.0.0-e2e-grpc`  | Semantic version for testing       |
| `BUILD_TIME`   | Auto-generated       | Auto-generated    | Auto-generated     | UTC timestamp when image was built |
| `SERVICE_NAME` | `simple-service-e2e` | `gin-service-e2e` | `grpc-service-e2e` | Service name via LDFlags           |
| `SERVICE_DIR`  | `simple-service`     | `gin-service`     | `grpc-service`     | Directory to build from            |

### Ports Mapping

| Component          | Host Port | Container Port | Purpose                  |
| :----------------- | :-------- | :------------- | :----------------------- |
| Simple Service API | 8081      | 8080           | HTTP API                 |
| Simple Metrics     | 9092      | 9090           | Prometheus scrape target |
| Gin Service API    | 8082      | 8080           | HTTP API with middleware |
| Gin Metrics        | 9093      | 9090           | Prometheus scrape target |
| gRPC Service       | 50051     | 50051          | gRPC API                 |
| gRPC Health        | 8083      | 8080           | Health check endpoint    |
| gRPC Metrics       | 9094      | 9090           | Prometheus scrape target |
| OTEL Collector     | 14318     | 4318           | OTLP HTTP receiver       |
| Prometheus         | 9099      | 9090           | Metrics query API        |
| Jaeger UI          | 16687     | 16686          | Trace visualization      |

**Note:** All services run in a dedicated Docker network (`e2e-network`) for isolation.

## Troubleshooting

### Port Already in Use

If you encounter port conflicts:

```bash
# Find and kill processes using required ports
fuser -k 8081/tcp 8082/tcp 8083/tcp 9092/tcp 9093/tcp 9094/tcp 9099/tcp 14318/tcp 16687/tcp 50051/tcp

# Or use lsof to identify processes
lsof -ti:8081,8082,8083,9092,9093,9094,9099,14318,16687,50051 | xargs kill -9
```

### Service Fails to Start

Check the service container logs:

```bash
# View service logs
docker logs simple-service
docker logs gin-service
docker logs grpc-service

# Check if containers are running
docker ps -a | grep -E "simple-service|gin-service|grpc-service"
```

### Build Fails

If Docker build fails, check:

```bash
# Verify go.mod versions are compatible
cat examples/simple-service/go.mod
cat examples/gin-service/go.mod
cat examples/grpc-service/go.mod

# Build manually to see detailed errors
cd e2e
docker compose build simple-service
docker compose build gin-service
docker compose build grpc-service
```

### No Traces in Jaeger

Ensure the OTEL Collector is running and accessible:

```bash
# Check OTEL Collector status
docker logs otel-collector

# Test OTLP endpoint
curl -v http://localhost:14318/v1/traces

# Check if services are sending traces
docker logs simple-service | grep -i trace
docker logs gin-service | grep -i trace
docker logs grpc-service | grep -i trace
```

### Metrics Not Scraped

Verify Prometheus is scraping the metrics endpoints:

```bash
# Check Prometheus targets
curl http://localhost:9099/api/v1/targets

# Check service metrics endpoints directly
curl http://localhost:9092/metrics | grep request_count
curl http://localhost:9093/metrics | grep gin_request_count
curl http://localhost:9094/metrics | grep grpc_request_count
```

### gRPC Service Issues

If gRPC service fails or tests skip gRPC calls:

```bash
# Install grpcurl (optional)
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Test gRPC service manually
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"name": "Test"}' localhost:50051 hello.HelloService/SayHello

# Check health endpoint
curl http://localhost:8083/health
```

### Script Compatibility Issues

If you see errors like "Bad substitution" or "not found":

```bash
# Ensure you're running with bash (or make executable)
bash e2e/run-e2e.sh

# Or make executable and run directly
chmod +x e2e/run-e2e.sh
./e2e/run-e2e.sh
```

The script uses POSIX-compatible syntax but requires bash features for colors and arrays.

## Cleanup

The test script automatically cleans up resources on exit (including interrupts). To manually clean
up:

```bash
# Stop and remove Docker containers
cd e2e
docker compose down

# Kill any remaining service processes
pkill -f simple-service
```

## CI/CD Integration

The E2E test is integrated into the GitHub Actions CI pipeline:

```yaml
- name: Run E2E tests
  run: |
    cd e2e
    chmod +x run-e2e.sh
    ./run-e2e.sh
```

This ensures every commit is validated against the full observability stack.

## Future Enhancements

Planned improvements for the E2E test suite:

- [ ] Add tests for error scenarios (failed requests, timeouts)
- [ ] Test custom span attributes and metrics labels
- [ ] Verify log correlation with traces
- [ ] Test different sampling rates
- [ ] Add performance benchmarks
- [ ] Test graceful shutdown and span flushing
- [ ] Add multi-service tracing scenarios

## Related Documentation

- [PLAN_E2E.md](PLAN_E2E.md) - Original implementation plan
- [README.md](README.md) - Library usage and configuration
- [OpenTelemetry Docs](https://opentelemetry.io/docs/) - OTEL specification
