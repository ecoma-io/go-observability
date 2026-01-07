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

# Ecoma Go Observability Library

A standardized library for Golang microservices at **Ecoma-io**, providing built-in
**Configuration**, **Structured Logging (Zap)**, and **Observability (OpenTelemetry)**.

## 🚀 Key Features

- **Unified Config**: Load configuration from multiple sources (.env, Environment Variables,
  LDFlags).
- **Structured Logging**: Robust JSON logging with Zap, automatically attaching service metadata.
- **Distributed Tracing**: OpenTelemetry Tracing integration via OTLP/HTTP.
- **Metrics**: Collect and expose system metrics using Prometheus (Pull model).
- **Build-time Metadata**: Support for injecting Version and Build Time from CI/CD.

---

## 📦 Installation

```bash
go get github.com/ecoma-io/observability
```

## 🛠 Usage

### 1. Initialize configuration and Logger

The library is designed to be easily embedded into each service's configuration struct.

```go
package main

import (
    "context"
    "github.com/ecoma-io/observability"
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

## 🏗 Build with LDFlags (Recommended)

To make full use of version management, use `-ldflags` to inject information into the binary at
build time:

```bash
SERVICE_NAME="order-service"
VERSION="v1.2.3"
BUILD_TIME=$(date +%FT%T%z)
MODULE_PATH="github.com/ecoma-io/observability"

go build -ldflags "-X '$MODULE_PATH.ServiceName=$SERVICE_NAME' \
                   -X '$MODULE_PATH.Version=$VERSION' \
                   -X '$MODULE_PATH.BuildTime=$BUILD_TIME'" \
         -o main .
```

## ⚙️ Environment Variables (Configuration)

| Variable                   | Default          | Description                                        |
| :------------------------- | :--------------- | :------------------------------------------------- |
| `SERVICE_NAME`             | **(Required)**   | Service name (Used as `service.name` in Otel/Log)  |
| `LOG_LEVEL`                | `info`           | Log level (`debug`, `info`, `warn`, `error`)       |
| `OTEL_ENDPOINT`            | `localhost:4318` | OTLP Collector address over HTTP (for Tracing)     |
| `METRICS_PORT`             | `9090`           | Port to run the Metrics server (Prometheus format) |
| `OTEL_TRACING_SAMPLE_RATE` | `1.0`            | Tracing sample rate (0.0 to 1.0)                   |

## 🧪 Testing

Run unit tests for the library:

```bash
go test -v ./...
```
