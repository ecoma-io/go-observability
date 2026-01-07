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

| Service            | Image                                  | Port(s)      | Purpose                                  |
| :----------------- | :------------------------------------- | :----------- | :--------------------------------------- |
| **simple-service** | Built from Dockerfile (multi-stage)    | 8081, 9092   | Test application with observability      |
| **otel-collector** | `otel/opentelemetry-collector-contrib` | 4318, 4317   | Receives OTLP data from services         |
| **jaeger**         | `jaegertracing/all-in-one`             | 16686, 14268 | Stores and visualizes distributed traces |
| **prometheus**     | `prom/prometheus`                      | 9090         | Scrapes and stores metrics               |

### 2. Test Application

**simple-service** is a minimal Go HTTP service built as a Docker image that demonstrates
observability library usage:

- **Endpoint**: `GET /ping` returns "pong"
- **Build Process**: Multi-stage Docker build with LDFlags injection
- **Features**:
  - Creates a trace span for each request
  - Increments a counter metric (`request_count_total`)
  - Logs request details with trace context (trace_id, span_id)
  - Includes build-time metadata (version, build_time) via LDFlags
  - Introduces random latency (0-100ms) for realistic tracing

**Build Arguments:**

- `VERSION=v1.0.0-e2e` - Semantic version injected at build time
- `BUILD_TIME` - UTC timestamp of when the image was built
- `SERVICE_NAME=simple-service` - Service identifier

### 3. Test Script

**run-e2e.sh** orchestrates the entire test flow:

1. Sets build-time environment variables (BUILD_TIME)
2. Builds Docker images with LDFlags using `docker compose build`
3. Starts all services with `docker compose up -d --build`
4. Waits for services to be ready
5. Sends HTTP requests to generate telemetry data
6. Verifies build metadata in service logs
7. Verifies traces in Jaeger
8. Verifies metrics in Prometheus
9. Cleans up all resources

**Key Features:**

- Uses absolute paths - can be run from any directory
- POSIX-compliant shell script (works with sh, bash, zsh)
- Automatic cleanup on exit or interrupt (Ctrl+C)
- Detailed verification with clear success/failure messages

## Running E2E Tests

### Prerequisites

- **Docker** and **Docker Compose** installed
- **Go 1.21+** installed
- Ports available: 8081, 9092, 9099, 14318, 16687

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
[1/3] Building and Starting Docker Infrastructure... ✓
  • Building Docker images with LDFlags
  • Starting all services
Waiting 15s for all services to start...
[2/3] Generating Load... ✓
  • Sending 5 HTTP requests
[3/4] Verifying Observability...
  • Checking build metadata... ✓ SUCCESS: Version metadata found!
  • Checking Jaeger... ✓ SUCCESS: Traces found in Jaeger!
  • Checking Prometheus... ✓ SUCCESS: Metrics found in Prometheus!
[4/4] Cleaning up...
E2E Tests Completed! ✓
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

The test queries the Jaeger API to confirm traces exist:

```bash
GET http://localhost:16687/api/traces?service=simple-service
```

**Success criteria**: Response contains `traceID` field

### Metrics Verification

The test queries the Prometheus API for the counter metric:

```bash
GET http://localhost:9099/api/v1/query?query=request_count_total
```

**Success criteria**:

- Response status is "success"
- Metric contains label `service="simple-service"`
- Counter value matches number of requests sent

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

The test service receives environment variables from docker-compose.yml:

| Variable        | Value                        | Description                           |
| :-------------- | :--------------------------- | :------------------------------------ |
| `SERVICE_NAME`  | `simple-service`             | Service identifier                    |
| `PORT`          | `8080` (mapped to host 8081) | HTTP server port                      |
| `METRICS_PORT`  | `9090` (mapped to host 9092) | Prometheus metrics endpoint port      |
| `OTEL_ENDPOINT` | `http://otel-collector:4318` | OpenTelemetry Collector HTTP endpoint |
| `LOG_LEVEL`     | `info`                       | Logging level                         |

### Build Arguments (Docker)

These are set at image build time via docker-compose.yml:

| Argument       | Value            | Purpose                            |
| :------------- | :--------------- | :--------------------------------- |
| `VERSION`      | `v1.0.0-e2e`     | Semantic version for testing       |
| `BUILD_TIME`   | Auto-generated   | UTC timestamp when image was built |
| `SERVICE_NAME` | `simple-service` | Service name injected via LDFlags  |

### Ports Mapping

| Component        | Host Port | Container Port | Purpose                  |
| :--------------- | :-------- | :------------- | :----------------------- |
| Simple Service   | 8081      | 8080           | HTTP API                 |
| Metrics Endpoint | 9092      | 9090           | Prometheus scrape target |
| OTEL Collector   | 14318     | 4318           | OTLP HTTP receiver       |
| Prometheus       | 9099      | 9090           | Metrics query API        |
| Jaeger UI        | 16687     | 16686          | Trace visualization      |

**Note:** All services run in a dedicated Docker network (`e2e-network`) for isolation.

## Troubleshooting

### Port Already in Use

If you encounter port conflicts:

```bash
# Find and kill processes using required ports
fuser -k 8081/tcp 9092/tcp 9099/tcp 14318/tcp 16687/tcp
```

### Service Fails to Start

Check the service container logs:

```bash
# View service logs
docker logs e2e-simple-service-1

# Check if container is running
docker ps -a | grep e2e-simple-service
```

### Build Fails

If Docker build fails, check:

```bash
# Verify go.mod versions are compatible
cat examples/simple-service/go.mod

# Build manually to see detailed errors
cd e2e
docker compose build simple-service
```

### No Traces in Jaeger

Ensure the OTEL Collector is running and accessible:

```bash
# Check OTEL Collector status
docker logs e2e-otel-collector-1

# Test OTLP endpoint
curl -v http://localhost:14318/v1/traces
```

### Metrics Not Scraped

Verify Prometheus is scraping the metrics endpoint:

```bash
# Check Prometheus targets
curl http://localhost:9099/api/v1/targets

# Check service metrics endpoint directly
curl http://localhost:9092/metrics | grep request_count
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
