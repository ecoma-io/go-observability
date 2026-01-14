package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/ecoma-io/go-observability"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
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

	logger.Info("Starting gin-service", "version", cfg.Version, "port", cfg.Port)

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

	// 4. Setup Gin with middleware
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Create middleware config to skip health and metrics endpoints
	middlewareConfig := &observability.ObservabilityMiddlewareConfig{
		SkipRoute: func(path string) bool {
			// Skip health checks and metrics endpoints
			return strings.HasPrefix(path, "/health") ||
				strings.HasPrefix(path, "/metrics") ||
				path == "/status"
		},
	}

	// Apply observability middleware with skip configuration
	for _, mw := range observability.GinMiddlewareWithConfig(logger, cfg.ServiceName, middlewareConfig) {
		router.Use(mw)
	}

	// Define routes
	// Health check endpoint (skipped from observability)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
		})
	})

	// Metrics endpoint (skipped from observability)
	router.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "metrics endpoint",
		})
	})

	// Status endpoint (skipped from observability)
	router.GET("/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Ping endpoint (tracked in observability)
	router.GET("/ping", pingHandler)
	router.GET("/users/:id", getUserHandler)
	router.GET("/panic", panicHandler)
	router.GET("/error", errorHandler)

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("Listening on", "addr", addr)
	if err := router.Run(addr); err != nil {
		logger.Fatal("Server error", "error", err)
	}
}

func pingHandler(c *gin.Context) {
	// Create span
	tracer := otel.Tracer("gin-service")
	ctx, span := tracer.Start(c.Request.Context(), "ping-handler")
	defer span.End()

	// Artificial delay
	ms := rand.Intn(100)
	time.Sleep(time.Duration(ms) * time.Millisecond)

	// Update metrics
	meter := otel.Meter("gin-service")
	counter, _ := meter.Int64Counter("gin_request_count_total")
	counter.Add(ctx, 1, metric.WithAttributes(
attribute.String("endpoint", "/ping"),
attribute.String("method", "GET"),
))

	c.JSON(http.StatusOK, gin.H{
"message":    "pong",
"latency_ms": ms,
})
}

func getUserHandler(c *gin.Context) {
	userID := c.Param("id")

	// Create span
	tracer := otel.Tracer("gin-service")
	ctx, span := tracer.Start(c.Request.Context(), "get-user")
	span.SetAttributes(attribute.String("user.id", userID))
	defer span.End()

	// Update metrics
	meter := otel.Meter("gin-service")
	counter, _ := meter.Int64Counter("gin_user_requests_total")
	counter.Add(ctx, 1)

	c.JSON(http.StatusOK, gin.H{
"user_id": userID,
"name":    "Test User",
"email":   fmt.Sprintf("user%s@example.com", userID),
})
}

func panicHandler(c *gin.Context) {
	// This will trigger the recovery middleware
	panic("intentional panic for e2e testing")
}

func errorHandler(c *gin.Context) {
	// Return a client error
	c.JSON(http.StatusBadRequest, gin.H{
"error":   "Bad Request",
"message": "Invalid parameters provided",
})
}
