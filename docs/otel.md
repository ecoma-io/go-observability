# OpenTelemetry

Initialize OTEL with:

```go
shutdown, err := observability.InitOtel(cfg.BaseConfig)
if err != nil { /* handle */ }
defer shutdown(context.Background())
```

Modes:

- `pull`: Prometheus exporter on `:9090/metrics`
- `push`: OTLP push to configured endpoint
- `hybrid`: both active

Tracer usage:

```go
tracer := observability.GetTracer("component-name")
ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()
```

## Metrics and Tracing modes (implementation details)

`InitOtel` configures tracing (OTLP/HTTP push by default) and metrics using one of three modes:

- `pull` (Prometheus): creates a Prometheus exporter and starts an internal HTTP server on
  `0.0.0.0:<MetricsPort>` serving `MetricsPath`.
- `push` (OTLP): creates an OTLP metrics exporter and registers a periodic reader to push metrics to
  `MetricsPushEndpoint`. Protocol can be `http` or `grpc` based on `MetricsProtocol`.
- `hybrid`: combines both pull and push behaviors.

If no readers are configured, the implementation falls back to a Prometheus exporter (pull).

## Shutdown behavior

`InitOtel` returns a shutdown function that:

- Force flushes the MeterProvider and TracerProvider.
- Shuts down the internal metrics HTTP server (if pull mode is enabled).
- Calls `Shutdown()` on the TracerProvider and MeterProvider and any push-specific shutdown
  functions.

Call the returned `shutdown(ctx)` during service termination (use a context with timeout for
graceful shutdown).

## Notes from code review

- The trace exporter and metric exporter constructors are configured with `WithInsecure()`; consider
  adding a config option to enable TLS or provide certificates for production deployments.
- Internal metrics server logs errors via `fmt.Printf` — in services prefer using
  `observability.Logger` to keep logs consistent.
- Ensure `METRICS_PUSH_ENDPOINT` is reachable from the runtime environment when using
  `push`/`hybrid` modes.

## Insecure / TLS configuration

For local development and tests it is common to run collectors without TLS. `InitOtel` now respects
two boolean config flags (and corresponding environment variables) to allow connecting to OTLP
endpoints without TLS:

- `OTEL_INSECURE` — when `true`, the OTLP trace exporter will use an insecure (non-TLS) connection
  to `OTEL_ENDPOINT`.
- `METRICS_INSECURE` — when `true`, OTLP metrics exporters (HTTP or gRPC) use insecure connections
  to `METRICS_PUSH_ENDPOINT`.

Set these to `true` only for local/dev/e2e environments. For production deployments prefer TLS
and/or mTLS and validate certificates.

## Metrics protocol and defaults

The `METRICS_PROTOCOL` config controls how metrics are pushed when using `push` or `hybrid` mode.
Supported values are `http` and `grpc`. When the value is empty the implementation defaults to
`http` for backwards compatibility.

## Shutdown ordering

The shutdown function returned by `InitOtel` performs orderly teardown to avoid data loss or
goroutine leaks. It first shuts down any push-specific readers (periodic readers), then
force-flushes the MeterProvider and TracerProvider, shuts down the pull metrics HTTP server (if
active), and finally shuts down the tracer and meter providers. This ordering ensures periodic
readers can flush their data before providers are torn down.
