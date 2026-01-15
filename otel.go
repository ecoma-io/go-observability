package observability

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
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

// InitOtel initializes OpenTelemetry with support for Tracing (Push)
// and Metrics (Pull/Push/Hybrid)
func InitOtel(cfg BaseConfig) (func(context.Context) error, error) {
	ctx := context.Background()

	// 1. Initialize Resource identifying the service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 2. Configure Tracing (Push model sending to Otel Collector)
	// Build options for trace exporter, respecting insecure config
	traceOpts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(cfg.OtelEndpoint),
	}
	if cfg.OtelInsecure {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
	}
	traceExp, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.OtelTracingSampleRate)),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(traceExp)),
	)
	otel.SetTracerProvider(tp)

	// 3. Configure Metrics based on MetricsMode
	var (
		mp              *sdkmetric.MeterProvider
		metricsServer   *http.Server
		metricsShutdown []func(context.Context) error
		readers         []sdkmetric.Reader
	)

	// Setup metrics exporter(s) based on mode
	if cfg.IsPull() {
		// Pull mode: Prometheus exporter
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, promExporter)

		// Setup HTTP server for pull metrics
		mux := http.NewServeMux()
		mux.Handle(cfg.MetricsPath, promhttp.Handler())

		metricsServer = &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.MetricsPort),
			Handler: mux,
		}

		// Try to bind the metrics port immediately so startup failures (e.g., port in use)
		// are returned to the caller instead of being logged asynchronously.
		ln, err := net.Listen("tcp", metricsServer.Addr)
		if err != nil {
			return nil, fmt.Errorf("failed to bind metrics server addr %s: %w", metricsServer.Addr, err)
		}

		go func() {
			if err := metricsServer.Serve(ln); err != nil && err != http.ErrServerClosed {
				fmt.Printf("Metrics server error: %v\n", err)
			}
		}()
	}

	if cfg.IsPush() {
		// Push mode: OTLP metrics exporter with protocol support (HTTP or gRPC)
		pushInterval := time.Duration(cfg.MetricsPushInterval) * time.Second
		
		// Create OTLP metric exporter based on protocol configuration
		protocol := strings.ToLower(strings.TrimSpace(cfg.MetricsProtocol))
		if protocol == "" {
			protocol = "http"
		}

		switch protocol {
		case "grpc":
			// Use gRPC protocol for OTLP metrics export
			grpcOpts := []otlpmetricgrpc.Option{
				otlpmetricgrpc.WithEndpoint(cfg.MetricsPushEndpoint),
			}
			if cfg.MetricsInsecure {
				grpcOpts = append(grpcOpts, otlpmetricgrpc.WithInsecure())
			}
			exp, err := otlpmetricgrpc.New(ctx, grpcOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP gRPC metrics exporter: %w", err)
			}

			reader := sdkmetric.NewPeriodicReader(exp,
				sdkmetric.WithInterval(pushInterval),
			)
			metricsShutdown = append(metricsShutdown, reader.Shutdown)
			readers = append(readers, reader)
		case "http":
			// Use HTTP protocol for OTLP metrics export
			httpOpts := []otlpmetrichttp.Option{
				otlpmetrichttp.WithEndpoint(cfg.MetricsPushEndpoint),
			}
			if cfg.MetricsInsecure {
				httpOpts = append(httpOpts, otlpmetrichttp.WithInsecure())
			}
			exp, err := otlpmetrichttp.New(ctx, httpOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP HTTP metrics exporter: %w", err)
			}

			reader := sdkmetric.NewPeriodicReader(exp,
				sdkmetric.WithInterval(pushInterval),
			)
			metricsShutdown = append(metricsShutdown, reader.Shutdown)
			readers = append(readers, reader)
		default:
			return nil, fmt.Errorf("invalid METRICS_PROTOCOL: %s", protocol)
		}
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

		// Try to bind the metrics port immediately so startup failures (e.g., port in use)
		// are returned to the caller instead of being logged asynchronously.
		ln, err := net.Listen("tcp", metricsServer.Addr)
		if err != nil {
			return nil, fmt.Errorf("failed to bind metrics server addr %s: %w", metricsServer.Addr, err)
		}

		go func() {
			if err := metricsServer.Serve(ln); err != nil && err != http.ErrServerClosed {
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

	// 4. Configure Global Propagator (W3C Trace Context & Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return a Shutdown function to clean up resources when service stops
	return func(ctx context.Context) error {
		var errs []string

		// Shutdown push-specific resources (readers/periodic readers) first
		for _, shutdown := range metricsShutdown {
			if err := shutdown(ctx); err != nil {
				errs = append(errs, fmt.Sprintf("push metrics shutdown error: %v", err))
			}
		}

		// ForceFlush Meter Provider to ensure all metrics are sent before shutdown
		if err := mp.ForceFlush(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("meter provider force flush error: %v", err))
		}

		// ForceFlush Tracer Provider to ensure all traces are sent before shutdown
		if err := tp.ForceFlush(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("tracer provider force flush error: %v", err))
		}

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

		if len(errs) > 0 {
			return fmt.Errorf("otel shutdown failures: %s", strings.Join(errs, "; "))
		}
		return nil
	}, nil
}

// GetTracer returns a tracer instance
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// GetMeter returns a meter instance
func GetMeter(name string) metric.Meter {
	return otel.Meter(name)
}
