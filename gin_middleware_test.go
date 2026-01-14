package observability

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestGinLogger(t *testing.T) {
	// Set Gin to Test Mode
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-service",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	tests := []struct {
		name       string
		path       string
		method     string
		statusCode int
	}{
		{
			name:       "Success_Request_200",
			path:       "/api/test",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
		},
		{
			name:       "Client_Error_404",
			path:       "/api/notfound",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
		},
		{
			name:       "Server_Error_500",
			path:       "/api/error",
			method:     http.MethodPost,
			statusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(GinLogger(logger))

			router.Handle(tt.method, tt.path, func(c *gin.Context) {
				c.Status(tt.statusCode)
			})

			req, _ := http.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestGinLoggerWithTraceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-trace",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	router := gin.New()
	router.Use(GinLogger(logger))

	router.GET("/trace", func(c *gin.Context) {
		// Create a span to simulate trace context
		tracer := otel.Tracer("test-tracer")
		ctx, span := tracer.Start(c.Request.Context(), "test-operation")
		defer span.End()

		c.Request = c.Request.WithContext(ctx)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/trace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGinRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-recovery",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	router := gin.New()
	router.Use(GinRecovery(logger))

	// Route that panics
	router.GET("/panic", func(c *gin.Context) {
		panic("something went wrong!")
	})

	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", w.Code)
	}

	// Check JSON response structure
	var errorResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errorResp.Error != "Internal Server Error" {
		t.Errorf("Expected error field 'Internal Server Error', got '%s'", errorResp.Error)
	}

	if errorResp.Path != "/panic" {
		t.Errorf("Expected path '/panic', got '%s'", errorResp.Path)
	}

	if errorResp.Message == "" {
		t.Error("Expected non-empty message field")
	}
}

func TestGinRecoveryWithTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-recovery-trace",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	router := gin.New()
	router.Use(GinRecovery(logger))

	// Route that panics with trace context
	router.GET("/panic-trace", func(c *gin.Context) {
		tracer := otel.Tracer("test-tracer")
		ctx, span := tracer.Start(c.Request.Context(), "panic-operation")
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		// Extract trace ID before panic
		spanContext := trace.SpanFromContext(ctx).SpanContext()
		if !spanContext.IsValid() {
			t.Log("Warning: No valid trace context in test")
		}

		panic("traced panic!")
	})

	req, _ := http.NewRequest(http.MethodGet, "/panic-trace", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	// Parse response
	var errorResp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	// Note: trace_id might be empty in test environment without full OTEL setup
	t.Logf("Response trace_id: %s", errorResp.TraceID)
}

func TestGinMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-combined",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	router := gin.New()
	// Apply both middleware using GinMiddleware helper
	for _, mw := range GinMiddleware(logger, cfg.ServiceName) {
		router.Use(mw)
	}

	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.GET("/error", func(c *gin.Context) {
		panic("test panic with combined middleware")
	})

	// Test normal request
	t.Run("Normal_Request", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/ok", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	// Test panic recovery
	t.Run("Panic_Recovery", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/error", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}

		var errorResp ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &errorResp); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}

		if errorResp.Error == "" {
			t.Error("Expected non-empty error field")
		}
	})
}
func TestGinLoggerWithExcludedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-gin-exclude",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	// Create middleware config with excluded paths
	middlewareCfg := &ObservabilityMiddlewareConfig{
		ExcludedPaths: []string{"/health", "/metrics"},
	}

	tests := []struct {
		name           string
		path           string
		method         string
		statusCode     int
		shouldBeMissed bool
	}{
		{
			name:           "Excluded_Health_Endpoint",
			path:           "/health",
			method:         http.MethodGet,
			statusCode:     http.StatusOK,
			shouldBeMissed: true,
		},
		{
			name:           "Excluded_Metrics_Endpoint",
			path:           "/metrics",
			method:         http.MethodGet,
			statusCode:     http.StatusOK,
			shouldBeMissed: true,
		},
		{
			name:           "Normal_Endpoint",
			path:           "/api/users",
			method:         http.MethodGet,
			statusCode:     http.StatusOK,
			shouldBeMissed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(GinLoggerWithConfig(logger, middlewareCfg))

			router.Handle(tt.method, tt.path, func(c *gin.Context) {
				c.Status(tt.statusCode)
			})

			req, _ := http.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, w.Code)
			}
		})
	}
}

func TestGinTracingWithExcludedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	middlewareCfg := &ObservabilityMiddlewareConfig{
		ExcludedPaths: []string{"/health", "/metrics"},
	}

	// Track which routes were reached
	routesCalled := make(map[string]bool)

	tests := []struct {
		name           string
		path           string
		shouldBeMissed bool
	}{
		{
			name:           "Excluded_Health",
			path:           "/health",
			shouldBeMissed: true,
		},
		{
			name:           "Excluded_Metrics",
			path:           "/metrics",
			shouldBeMissed: true,
		},
		{
			name:           "Normal_Request",
			path:           "/api/data",
			shouldBeMissed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routesCalled[tt.path] = false
			
			router := gin.New()
			router.Use(GinTracingWithConfig("test-service", middlewareCfg))

			router.GET(tt.path, func(c *gin.Context) {
				// Mark that route was reached
				routesCalled[tt.path] = true
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Verify the handler was called for both skipped and non-skipped paths
			if !routesCalled[tt.path] {
				t.Errorf("Expected handler to be called for path %s", tt.path)
			}
		})
	}
}

func TestGinMiddlewareWithSkipRoutePredicate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-skip-predicate",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	// Create middleware config with skip predicate
	middlewareCfg := &ObservabilityMiddlewareConfig{
		SkipRoute: func(path string) bool {
			// Skip paths that start with /health or /metrics
			return strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/metrics")
		},
	}

	tests := []struct {
		name           string
		path           string
		shouldBeMissed bool
	}{
		{
			name:           "Health_Check",
			path:           "/health",
			shouldBeMissed: true,
		},
		{
			name:           "Health_Detailed",
			path:           "/health/detailed",
			shouldBeMissed: true,
		},
		{
			name:           "Metrics_Endpoint",
			path:           "/metrics",
			shouldBeMissed: true,
		},
		{
			name:           "Metrics_Prometheus",
			path:           "/metrics/prometheus",
			shouldBeMissed: true,
		},
		{
			name:           "API_Request",
			path:           "/api/users",
			shouldBeMissed: false,
		},
		{
			name:           "Health_Like_Path",
			path:           "/api/health-check",
			shouldBeMissed: false, // Contains /health but doesn't start with it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			for _, mw := range GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareCfg) {
				router.Use(mw)
			}

			router.GET(tt.path, func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &BaseConfig{
		ServiceName: "test-backward-compat",
		Version:     "v1.0.0",
		LogLevel:    "info",
	}
	logger := NewLogger(cfg)

	// Test that old API still works without config
	router := gin.New()
	for _, mw := range GinMiddleware(logger, cfg.ServiceName) {
		router.Use(mw)
	}

	router.GET("/api/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}