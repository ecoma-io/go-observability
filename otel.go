package observability

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
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

// InitOtel khởi tạo OpenTelemetry với Tracing (Push) và Metrics (Pull via Prometheus)
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
		otlptracehttp.WithInsecure(), // Sử dụng WithTLSCredentials() cho production
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

	// 3. Cấu hình Metrics (Pull model thông qua Prometheus exporter)
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(mp)

	// 4. Khởi tạo HTTP Server nội bộ để phục vụ Prometheus Scraping
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.MetricsPort), // Listen on all interfaces
		Handler: mux,
	}

	// Chạy Metrics Server trong goroutine riêng
	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	// 5. Cấu hình Global Propagator (W3C Trace Context & Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Trả về hàm Shutdown để dọn dẹp tài nguyên khi dừng service
	return func(ctx context.Context) error {
		var errs []string

		// Shutdown Metrics Server
		if err := metricsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("metrics server shutdown error: %v", err))
		}

		// Shutdown Tracer Provider
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("tracer provider shutdown error: %v", err))
		}

		// Shutdown Meter Provider
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("meter provider shutdown error: %v", err))
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