package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/ecoma-io/go-observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Config struct {
	observability.BaseConfig
	Port int `env:"PORT" env-default:"8080"`
}

var (
	logger *observability.Logger
)

func main() {
	// 1. Load Config
	var cfg Config
	if err := observability.LoadCfg(&cfg); err != nil {
		panic(err)
	}

	// 2. Init Logger
	logger = observability.NewLogger(&cfg.BaseConfig)
	defer logger.Sync()

	logger.Info("Starting simple-service", "version", cfg.Version)

	// 3. Init Otel
	shutdown, err := observability.InitOtel(cfg.BaseConfig)
	if err != nil {
		logger.Fatal("Failed to init Otel", "error", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			logger.Error("Failed to shutdown Otel", "error", err)
		}
	}()

	// 4. HTTP Server
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", pingHandler)

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Listening on", "addr", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal("Server error", "error", err)
	}
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create Span
	tracer := observability.GetTracer("simple-service")
	ctx, span := tracer.Start(ctx, "ping-handler")
	defer span.End()

	// Add some artificial delay
	ms := rand.Intn(100)
	time.Sleep(time.Duration(ms) * time.Millisecond)

	// Update Metrics
	meter := observability.GetMeter("simple-service")
	counter, _ := meter.Int64Counter("request_count_total")
	counter.Add(ctx, 1, metric.WithAttributes(attribute.String("endpoint", "/ping")))

	// Log with trace context
	sc := span.SpanContext()
	logger.Info("Ping received",
		"trace_id", sc.TraceID().String(),
		"span_id", sc.SpanID().String(),
		"latency_ms", ms,
	)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}
