module github.com/ecoma-io/go-observability/examples/gin-example

go 1.25.5

replace github.com/ecoma-io/go-observability => ../..

require (
	github.com/ecoma-io/go-observability v0.0.0
	github.com/gin-gonic/gin v1.11.0
	go.opentelemetry.io/otel v1.39.0
)
