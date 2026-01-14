package observability

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityMiddlewareConfig holds configuration for observability middleware
type ObservabilityMiddlewareConfig struct {
	// ExcludedPaths is a list of paths to exclude from observability tracking
	ExcludedPaths []string
	// SkipRoute is a custom predicate function to determine if a route should be skipped
	// If both ExcludedPaths and SkipRoute are set, SkipRoute takes precedence
	SkipRoute func(path string) bool
}

// shouldSkipRoute checks if a path should be skipped based on configuration
func (c *ObservabilityMiddlewareConfig) shouldSkipRoute(path string) bool {
	if c == nil {
		return false
	}

	// Custom predicate takes precedence
	if c.SkipRoute != nil {
		return c.SkipRoute(path)
	}

	// Check excluded paths
	for _, excluded := range c.ExcludedPaths {
		if path == excluded {
			return true
		}
	}

	return false
}

// GinTracing middleware creates OpenTelemetry spans for HTTP requests
func GinTracing(serviceName string) gin.HandlerFunc {
	return GinTracingWithConfig(serviceName, nil)
}

// GinTracingWithConfig middleware creates OpenTelemetry spans for HTTP requests with skip configuration
func GinTracingWithConfig(serviceName string, cfg *ObservabilityMiddlewareConfig) gin.HandlerFunc {
	tracer := otel.Tracer("gin-server")
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Check if this path should be skipped
		if cfg.shouldSkipRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Extract trace context from incoming headers (W3C Trace Context)
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Create a span for this request
		ctx, span := tracer.Start(ctx, c.Request.Method+" "+c.Request.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Store the context with span for downstream use
		c.Request = c.Request.WithContext(ctx)

		// Inject trace_id into response header for client tracking
		spanContext := span.SpanContext()
		if spanContext.HasTraceID() {
			c.Header("X-Trace-ID", spanContext.TraceID().String())
		}

		c.Next()
	}
}

// GinLogger middleware logs HTTP requests with OpenTelemetry trace context
func GinLogger(logger *Logger) gin.HandlerFunc {
	return GinLoggerWithConfig(logger, nil)
}

// GinLoggerWithConfig middleware logs HTTP requests with OpenTelemetry trace context and skip configuration
func GinLoggerWithConfig(logger *Logger, cfg *ObservabilityMiddlewareConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if this path should be skipped
		if cfg.shouldSkipRoute(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Extract trace context if available
		span := trace.SpanFromContext(c.Request.Context())
		spanContext := span.SpanContext()
		traceID := spanContext.TraceID().String()
		spanID := spanContext.SpanID().String()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// Build log fields
		fields := []interface{}{
			"status", statusCode,
			"method", method,
			"path", path,
			"query", query,
			"ip", clientIP,
			"latency_ms", latency.Milliseconds(),
			"user_agent", c.Request.UserAgent(),
		}

		// Add trace context if present
		if traceID != "" && traceID != "00000000000000000000000000000000" {
			fields = append(fields, "trace_id", traceID)
		}
		if spanID != "" && spanID != "0000000000000000" {
			fields = append(fields, "span_id", spanID)
		}

		// Add error message if present
		if errorMessage != "" {
			fields = append(fields, "error", errorMessage)
		}

		// Log based on status code
		switch {
		case statusCode >= 500:
			logger.Error("HTTP Server Error", fields...)
		case statusCode >= 400:
			logger.Warn("HTTP Client Error", fields...)
		default:
			logger.Info("HTTP Request", fields...)
		}
	}
}

// ErrorResponse represents the JSON error response structure
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	TraceID string `json:"trace_id,omitempty"`
	Path    string `json:"path,omitempty"`
}

// GinRecovery middleware recovers from panics and returns structured error responses
func GinRecovery(logger *Logger) gin.HandlerFunc {
	return GinRecoveryWithConfig(logger, nil)
}

// GinRecoveryWithConfig middleware recovers from panics and returns structured error responses with skip configuration
func GinRecoveryWithConfig(logger *Logger, cfg *ObservabilityMiddlewareConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check if this path should be skipped
				if cfg.shouldSkipRoute(c.Request.URL.Path) {
					// Re-panic for other error handlers to catch
					panic(err)
				}

				// Extract trace context
				span := trace.SpanFromContext(c.Request.Context())
				spanContext := span.SpanContext()
				traceID := spanContext.TraceID().String()

				// Get stack trace
				stack := string(debug.Stack())

				// Log the panic with full context
				logger.Error("Panic recovered",
					"error", fmt.Sprintf("%v", err),
					"trace_id", traceID,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"stack", stack,
				)

				// Prepare error response
				errorResp := ErrorResponse{
					Error:   "Internal Server Error",
					Message: "An unexpected error occurred. Please try again later.",
					Path:    c.Request.URL.Path,
				}

				// Include trace_id if available (for debugging)
				if traceID != "" && traceID != "00000000000000000000000000000000" {
					errorResp.TraceID = traceID
				}

				// Return 500 error
				c.AbortWithStatusJSON(http.StatusInternalServerError, errorResp)
			}
		}()

		c.Next()
	}
}

// GinMiddleware combines tracing, recovery, and logging middleware
// Usage: router.Use(observability.GinMiddleware(logger, "service-name")...)
func GinMiddleware(logger *Logger, serviceName string) []gin.HandlerFunc {
	return GinMiddlewareWithConfig(logger, serviceName, nil)
}

// GinMiddlewareWithConfig combines tracing, recovery, and logging middleware with skip configuration
// Usage: router.Use(observability.GinMiddlewareWithConfig(logger, "service-name", cfg)...)
func GinMiddlewareWithConfig(logger *Logger, serviceName string, cfg *ObservabilityMiddlewareConfig) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		GinTracingWithConfig(serviceName, cfg),
		GinRecoveryWithConfig(logger, cfg),
		GinLoggerWithConfig(logger, cfg),
	}
}
