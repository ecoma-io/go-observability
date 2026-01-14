package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// InitOtel khởi tạo OpenTelemetry với hỗ trợ Tracing (Push) và Metrics (Pull/Push/Hybrid)
func InitOtel(cfg BaseConfig) (func(context.Context) error, error) {
	ctx := context.Background()

	// 1. Khởi tạo Resource định danh dịch vụ
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 2. Cấu hình Tracing (Push model gửi đến Otel Collector)
	traceExp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OtelEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.OtelTracingSampleRate)),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExp)),
	)
	otel.SetTracerProvider(tp)

	// 3. Cấu hình Metrics dựa trên MetricsMode
	var (
		mp              *sdkmetric.MeterProvider
		metricsServer   *http.Server
		metricsShutdown []func(context.Context) error
		readers         []sdkmetric.Reader
	)

	// Setup metrics exporter(s) dựa trên mode
	if cfg.IsPull() {
		// Pull mode: Prometheus exporter
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, promExporter)

		// Setup HTTP server cho pull metrics
		mux := http.NewServeMux()
		mux.Handle(cfg.MetricsPath, promhttp.Handler())

		metricsServer = &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.MetricsPort),
			Handler: mux,
		}

		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Metrics server error: %v\n", err)
			}
		}()
	}

	if cfg.IsPush() {
		// Push mode: OTLP metrics exporter
		pushInterval := time.Duration(cfg.MetricsPushInterval) * time.Second
		otlpMetricsExp, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(cfg.MetricsPushEndpoint),
			otlpmetrichttp.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metrics exporter: %w", err)
		}

		reader := sdkmetric.NewPeriodicReader(otlpMetricsExp,
			sdkmetric.WithInterval(pushInterval),
		)
		metricsShutdown = append(metricsShutdown, reader.Shutdown)
		readers = append(readers, reader)
	}

	// If no readers configured, default to pull mode
	if len(readers) == 0 {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, promExporter)

		mux := http.NewServeMux()
		mux.Handle(cfg.MetricsPath, promhttp.Handler())

		metricsServer = &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.MetricsPort),
			Handler: mux,
		}

		go func() {
			if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Metrics server error: %v\n", err)
			}
		}()
	}

	// Create MeterProvider with all readers
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}
	for _, r := range readers {
		opts = append(opts, sdkmetric.WithReader(r))
	}
	mp = sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(mp)

	// 4. Cấu hình Global Propagator (W3C Trace Context & Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Trả về hàm Shutdown để dọn dẹp tài nguyên khi dừng service
	return func(ctx context.Context) error {
		var errs []string

		// Shutdown Metrics Server (if pull mode enabled)
		if metricsServer != nil {
			if err := metricsServer.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Sprintf("metrics server shutdown error: %v", err))
			}
		}

		// Shutdown Tracer Provider
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("tracer provider shutdown error: %v", err))
		}

		// Shutdown Meter Provider
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("meter provider shutdown error: %v", err))
		}

		// Shutdown push-specific resources
		for _, shutdown := range metricsShutdown {
			if err := shutdown(ctx); err != nil {
				errs = append(errs, fmt.Sprintf("push metrics shutdown error: %v", err))
			}
		}

		if len(errs) > 0 {
			return fmt.Errorf("otel shutdown failures: %s", strings.Join(errs, "; "))
		}
		return nil
	}, nil
}

// GetTracer trả về một tracer instance
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// GetMeter trả về một meter instance
func GetMeter(name string) metric.Meter {
	return otel.Meter(name)
}
