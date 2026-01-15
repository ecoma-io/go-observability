# Changelog

## [1.5.0](https://github.com/ecoma-io/go-observability/compare/v1.4.0...v1.5.0) (2026-01-15)


### Features

* add support for insecure OTLP and metrics configurations ([16d3984](https://github.com/ecoma-io/go-observability/commit/16d39841fbcc2abf458518ce86381ffb46b35e8a))

## [1.4.0](https://github.com/ecoma-io/go-observability/compare/v1.3.0...v1.4.0) (2026-01-15)


### Features

* bind metrics server port immediately to handle startup failures ([de1e085](https://github.com/ecoma-io/go-observability/commit/de1e0857998f3e27769a74f26e49fdff814267f9))
* validate METRICS_PORT range  is a valid port (1-65535) ([6b03f86](https://github.com/ecoma-io/go-observability/commit/6b03f867ec80660644cb803849567ff22649aa9d))

## [1.3.0](https://github.com/ecoma-io/go-observability/compare/v1.2.1...v1.3.0) (2026-01-14)


### Features

* add native OTLP metric export ([7b3fef4](https://github.com/ecoma-io/go-observability/commit/7b3fef43932a01a409e8d53b4729ab719e771f0e))
* add route skipping capabilities for Gin middleware ([b3090c4](https://github.com/ecoma-io/go-observability/commit/b3090c47884b39adc8017799788e6ecdf07824a4)), closes [#8](https://github.com/ecoma-io/go-observability/issues/8)
* add support for hybrid metrics mode ([a31ca0f](https://github.com/ecoma-io/go-observability/commit/a31ca0f3ec6866aac462150bdfcd2798b1975cfe)), closes [#5](https://github.com/ecoma-io/go-observability/issues/5)

## [1.2.1](https://github.com/ecoma-io/go-observability/compare/v1.2.0...v1.2.1) (2026-01-10)


### Bug Fixes

* downgrade go version from 1.25.5 to 1.24 ([7d2ed92](https://github.com/ecoma-io/go-observability/commit/7d2ed92eea8b261405bb582011930b3d83aa77bd))

## [1.2.0](https://github.com/ecoma-io/go-observability/compare/v1.1.0...v1.2.0) (2026-01-09)


### Features

* add GinTracing middleware for OpenTelemetry support ([82c0257](https://github.com/ecoma-io/go-observability/commit/82c0257864ab713ba165a6d8b2eaa4ecd32ab6d0))

## [1.1.0](https://github.com/ecoma-io/go-observability/compare/v1.0.0...v1.1.0) (2026-01-09)


### Features

* Add Gin middleware intergrate observability ([f60d9b0](https://github.com/ecoma-io/go-observability/commit/f60d9b07f2376d12271f51f98918287cfd891b0f))


### Bug Fixes

* rename observability to go-observability ([648ba71](https://github.com/ecoma-io/go-observability/commit/648ba7146e7316b8e7878ee535c56340fb6955cd))

## 1.0.0 (2026-01-07)


### Features

* implement configuration, logging, and OTEL integration ([22e1ddc](https://github.com/ecoma-io/go-observability/commit/22e1ddcf79c37659046690723d8d6f8900303c14))
