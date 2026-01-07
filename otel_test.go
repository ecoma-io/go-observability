package observability

import (
	"context"
	"testing"
	"time"
)

func TestInitOtel(t *testing.T) {
	// Pick a random port to avoid conflicts during tests
	// or use a high port.
	cfg := BaseConfig{
		ServiceName:           "test-otel",
		Version:               "1.0.0",
		OtelEndpoint:          "localhost:4318",
		OtelTracingSampleRate: 1.0,
		MetricsPort:           19090, // Test port
	}

	t.Run("Init Success", func(t *testing.T) {
		shutdown, err := InitOtel(cfg)
		if err != nil {
			t.Fatalf("InitOtel failed: %v", err)
		}
		if shutdown == nil {
			t.Fatal("shutdown function is nil")
		}

		// Verify GetTracer / GetMeter work
		tracer := GetTracer("test-tracer")
		if tracer == nil {
			t.Error("GetTracer returned nil")
		}

		meter := GetMeter("test-meter")
		if meter == nil {
			t.Error("GetMeter returned nil")
		}

		// Create a span to ensure provider is working
		_, span := tracer.Start(context.Background(), "test-span")
		span.End()

		// Allow some time for things to settle if needed, though strictly not necessary for unit test
		time.Sleep(10 * time.Millisecond)

		// Test Shutdown
		err = shutdown(context.Background())
		if err != nil {
			t.Errorf("shutdown returned error: %v", err)
		}
	})
}
