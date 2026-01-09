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

// GinTracing middleware creates OpenTelemetry spans for HTTP requests
func GinTracing(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer("gin-server")
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
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
	return func(c *gin.Context) {
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
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
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
	return []gin.HandlerFunc{
		GinTracing(serviceName),
		GinRecovery(logger),
		GinLogger(logger),
	}
}
