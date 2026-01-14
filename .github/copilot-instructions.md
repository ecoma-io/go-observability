# Go Observability - AI Coding Agent Instructions

## Project Overview

**go-observability** is a standardized library for Go microservices providing configuration
management, structured logging (Zap), and OpenTelemetry integration (tracing & metrics). It's
designed to be embedded into service `BaseConfig` structs and offers ready-to-use Gin middleware and
gRPC interceptors.

### Architecture

```
Config Loading → Logger Init → OTEL Init (Tracing + Metrics)
                     ↓
           Service Implementation
                ↓ (uses middleware/interceptors)
           Traces → OTLP Collector
           Metrics → Prometheus or OTLP
           Logs → JSON to stdout
```

## Key Components & Patterns

### 1. Configuration Pattern (`config.go`)

- **BaseConfig embedding**: Services extend `BaseConfig` with custom fields
  ```go
  type Config struct {
      observability.BaseConfig
      DatabaseURL string `env:"DATABASE_URL"`
  }
  ```
- **Priority order**: LDFlags (build-time) > .env file > Environment variables
- **Load with**: `observability.LoadCfg(&cfg)` - validates and injects metadata
- **Modes**: `IsPull()`, `IsPush()`, `IsHybrid()` check metrics behavior

### 2. Logging Pattern (`logger.go`)

- **Zap-based** JSON structured logging with service metadata auto-attached
- **Create**: `observability.NewLogger(&cfg.BaseConfig)`
- **API**: `logger.Info(msg, key1, val1, key2, val2)` (sugared interface)
- **Always defer**: `defer logger.Sync()` to flush buffers

### 3. OpenTelemetry Pattern (`otel.go`)

- **Initialize**: `shutdown, err := observability.InitOtel(cfg.BaseConfig)`
- **Always defer shutdown**: `defer shutdown(context.Background())`
- **Resources**: Service name & version auto-attached via `semconv.ServiceName()`
- **Tracing**: Push model via OTLP/HTTP (configurable endpoint, sample rate)
- **Metrics modes**:
  - `pull`: Prometheus exporter on `:9090/metrics`
  - `push`: OTLP push periodically
  - `hybrid`: Both active
- **In code**:
  ```go
  tracer := observability.GetTracer("component-name")
  ctx, span := tracer.Start(ctx, "operation-name")
  defer span.End()
  ```

### 4. Middleware & Interceptors

- **Gin**: `observability.GinTracing(serviceName)` or
  `GinTracingWithConfig(serviceName, &ObservabilityMiddlewareConfig{ExcludedPaths: []string{"/health"}})`
  - Auto-creates spans, logs requests, recovers panics
- **gRPC**: `observability.GrpcUnaryServerInterceptor(logger)` for unary calls + streaming
  interceptor
  - Captures gRPC status codes, extracts trace context from metadata
- **Both**: Extract trace context via propagation from incoming requests

### 5. Build Metadata Injection (`metadata.go`)

- Global vars: `ServiceName`, `Version`, `BuildTime`
- **Accessed via functions**: `GetServiceName()`, `GetVersion()`, `GetBuildTime()`
- **Injected at build time**:
  ```bash
  go build \
    -ldflags="-X github.com/ecoma-io/go-observability.ServiceName=my-service \
              -X github.com/ecoma-io/go-observability.Version=v1.2.3 \
              -X github.com/ecoma-io/go-observability.BuildTime=2025-01-14"
  ```
- **Used by**: `observability.LoadCfg()` applies via `MetadataSetter` interface

## Testing Patterns

### Unit Tests

- Mock environment with `os.Setenv()` and backup/restore original values
- Rename `.env` to `.env.testbak` to force `ReadEnv` path (see `config_test.go`)
- Use table-driven tests with `t.Run()` subtests

### E2E Tests

- **Docker Compose stack**: Jaeger, OTEL Collector, Prometheus in `e2e/` directory
- **Build with LDFlags**: Inject version/time at compile time (see `e2e/Dockerfile`)
- **Run with**: `bash e2e/run-e2e.sh` (orchestrates all services + traffic generation)
- **Verify**: Traces in Jaeger, metrics in Prometheus, logs in JSON format

## Common Development Tasks

### Adding a feature to core library

1. Create test file (`*_test.go`) with failing test first
2. Implement in corresponding `.go` file
3. Ensure `observability.Logger` is used for consistency
4. Update examples if affecting public API

### Extending BaseConfig

- Add field with `env:` tag for environment variable binding
- Add validation in `finalizeAndValidate()` function
- Document env var in README

### Creating a new service example

1. Place in `examples/{service-name}/` directory
2. Extend `BaseConfig` for service-specific config
3. Follow init pattern: LoadCfg → NewLogger → InitOtel
4. Use `GinTracing()` middleware or `GrpcUnaryServerInterceptor()` interceptor
5. Create `.env` for local development, `go.mod` for dependencies

### Integrating into existing service

1. Add `go get github.com/ecoma-io/go-observability`
2. Embed `BaseConfig` in service config
3. Call `observability.LoadCfg()`, `NewLogger()`, `InitOtel()`
4. Wrap HTTP handlers with `GinTracing()` or gRPC with interceptor

## Critical Workflows

### Run all unit tests

```bash
go test ./... -v
```

### Run E2E tests (Docker required)

```bash
bash e2e/run-e2e.sh
```

### Local development with example service

```bash
cd examples/gin-service
export SERVICE_NAME=my-gin-service LOG_LEVEL=debug
go run main.go
# Metrics available at http://localhost:9090/metrics
```

### Check/Format code

- Code style: Standard Go conventions
- Linting: Use project's `.lefthook.yml` hooks
- Format: `gofmt -w .` (standard tooling)

## Important Implementation Notes

- **Context propagation**: Always pass `ctx` through function calls to maintain trace continuity
- **Panic recovery**: Middleware auto-recovers; log panic details via span attributes
- **Resource exhaustion**: Metrics readers in `hybrid` mode may increase memory - use
  sampling/aggregation
- **OTEL endpoint**: Defaults to `localhost:4318`; set `OTEL_ENDPOINT` for external collectors
- **Sample rate**: `OTEL_TRACING_SAMPLE_RATE=0.1` reduces 90% of trace overhead (default: 1.0)
- **JSON logs**: Always check timestamp format (ISO8601) and service metadata presence

## File Structure Reference

```
.github/           # CI/CD workflows
config.go          # Configuration + env loading
logger.go          # Zap-based logging wrapper
otel.go            # OpenTelemetry init (traces + metrics)
gin_middleware.go  # HTTP middleware for Gin framework
grpc_interceptor.go # gRPC unary/streaming interceptors
metadata.go        # Build-time metadata (version, service name)
examples/          # Reference implementations (Gin, gRPC, simple)
e2e/               # End-to-end test suite with Docker Compose
```

## When Uncertain

- Check example implementations in `examples/` directory for integration patterns
- Refer to test files (`*_test.go`) for expected behavior and edge cases
- Review recent changes via `CHANGELOG.md` for architectural decisions
- Existing patterns take precedence over generic best practices
