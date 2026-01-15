<!-- markdownlint-disable -->
<div align="center">

![GitHub Release](https://img.shields.io/github/v/release/ecoma-io/go-observability)
![Coveralls](https://img.shields.io/coverallsCoverage/github/ecoma-io/go-observability)
![GitHub License](https://img.shields.io/github/license/ecoma-io/go-observability)
[![Open in DevContainer](https://img.shields.io/badge/Open-DevContainer-blue.svg)](https://vscode.dev/redirect?url=vscode://ms-vscode-remote.remote-containers/cloneInVolume?url=https://github.com/ecoma-io/services)

</div>

<p align="center">
  <img src="docs/assets/logo.svg" alt="go-observability" width="120" />
</p>

# Go Observability

Lightweight observability building blocks for Go microservices.

go-observability provides a compact, opinionated set of helpers for:

- Configuration loading and validation (`BaseConfig`, environment and LDFlags support)
- Structured JSON logging powered by Zap
- OpenTelemetry integration (tracing + metrics) with sensible defaults
- Gin middleware and gRPC interceptors for tracing, logging and panic recovery

## Documentation

Comprehensive documentation and examples are published in the `docs/` folder and the project site:

- Docs folder: [docs/index.md](docs/index.md)
- Document site:
  [https://ecoma-io.github.io/go-observability](https://ecoma-io.github.io/go-observability/)
- Local preview: run `task docs:serve` (see `Taskfile.yml`)

## Quick start

Install the library and use it in your service by embedding `observability.BaseConfig` and calling
`observability.LoadCfg`, `observability.NewLogger` and `observability.InitOtel` during startup. See
full examples in the `examples/` directory.

## Examples

Hands-on examples are available under `examples/`:

- `examples/gin-example` — Gin middleware demo (route skipping, panic recovery, metrics)
- `examples/gin-service` — Simple Gin service
- `examples/grpc-service` — gRPC server with interceptors and grpcurl examples
- `examples/simple-service` — Minimal example with `/ping`

## Development & docs workflow

Use the provided Taskfile for common tasks:

```bash
# Install docs tools locally (recommended: pipx)
task docs-install-pipx

# Serve docs locally
task docs-serve

# Run linters and tests
task lint
task analysis
task test
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and `docs/contributing.md` for contribution guidelines, PR
templates and code review expectations.

## License

This project is licensed under the terms in the `LICENSE` file.
