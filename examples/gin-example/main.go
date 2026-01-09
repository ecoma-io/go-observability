package main

import (
	"context"
	"net/http"
	"time"

	"github.com/ecoma-io/go-observability"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
)

func main() {
	// 1. Define the service config struct by embedding BaseConfig
	type Config struct {
		observability.BaseConfig
		Port string `env:"PORT" env-default:"8080"`
	}

	var cfg Config

	// 2. Load configuration (Order: LDFlags > .env > Environment Variables)
	if err := observability.LoadCfg(&cfg); err != nil {
		panic(err)
	}

	// 3. Initialize Logger
	logger := observability.NewLogger(&cfg.BaseConfig)
	defer logger.Sync()

	logger.Info("Service started", "version", cfg.Version, "port", cfg.Port)

	// 4. Initialize OpenTelemetry (Tracing & Metrics)
	shutdown, err := observability.InitOtel(cfg.BaseConfig)
	if err != nil {
		logger.Fatal("Failed to init Otel", "error", err)
	}
	defer shutdown(context.Background())

	// 5. Setup Gin with observability middleware
	router := gin.New()

	// Apply middleware: Tracing, Recovery, then Logger
	for _, mw := range observability.GinMiddleware(logger, cfg.ServiceName) {
		router.Use(mw)
	}

	// Alternative: apply middleware individually
	// router.Use(observability.GinRecovery(logger))
	// router.Use(observability.GinLogger(logger))

	// 6. Define routes
	router.GET("/ping", func(c *gin.Context) {
		// Create a span for this operation
		tracer := otel.Tracer("api-handler")
		ctx, span := tracer.Start(c.Request.Context(), "ping-handler")
		defer span.End()

		// Simulate some work
		time.Sleep(10 * time.Millisecond)

		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
			"service": cfg.ServiceName,
			"version": cfg.Version,
		})

		// Update request context for proper trace propagation
		c.Request = c.Request.WithContext(ctx)
	})

	router.GET("/users/:id", func(c *gin.Context) {
		userID := c.Param("id")

		// Create a span
		tracer := otel.Tracer("user-handler")
		ctx, span := tracer.Start(c.Request.Context(), "get-user")
		defer span.End()

		// Record a metric
		meter := otel.Meter("user-service")
		counter, _ := meter.Int64Counter("user_requests_total")
		counter.Add(ctx, 1)

		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"name":    "John Doe",
		})
	})

	// Example route that triggers an error
	router.GET("/error", func(c *gin.Context) {
		// This will be caught by GinRecovery middleware
		panic("something went wrong!")
	})

	// Example route with client error
	router.GET("/not-found", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Resource not found",
		})
	})

	// 7. Start server
	addr := ":" + cfg.Port
	logger.Info("Server listening", "address", addr)

	if err := router.Run(addr); err != nil {
		logger.Fatal("Server failed to start", "error", err)
	}
}
