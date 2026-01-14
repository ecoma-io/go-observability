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
		MetricsMode:           "pull",
		MetricsPath:           "/metrics",
	}

	t.Run("Init Success with Pull Mode", func(t *testing.T) {
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

	t.Run("Init Success with Push Mode", func(t *testing.T) {
		cfgPush := cfg
		cfgPush.MetricsPort = 19091
		cfgPush.MetricsMode = "push"
		cfgPush.MetricsPushEndpoint = "localhost:4318"
		cfgPush.MetricsPushInterval = 30

		shutdown, err := InitOtel(cfgPush)
		if err != nil {
			t.Fatalf("InitOtel with push mode failed: %v", err)
		}
		if shutdown == nil {
			t.Fatal("shutdown function is nil")
		}

		// Create a meter to test push exporter
		meter := GetMeter("test-meter-push")
		if meter == nil {
			t.Error("GetMeter returned nil for push mode")
		}

		// Allow some time for push to initialize
		time.Sleep(10 * time.Millisecond)

		// Test Shutdown (may error due to no collector, but shouldn't panic)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdown(ctx) // Ignore error as collector may not be running
	})

	t.Run("Init Success with Hybrid Mode", func(t *testing.T) {
		cfgHybrid := cfg
		cfgHybrid.MetricsPort = 19092
		cfgHybrid.MetricsMode = "hybrid"
		cfgHybrid.MetricsPushEndpoint = "localhost:4318"
		cfgHybrid.MetricsPushInterval = 30

		shutdown, err := InitOtel(cfgHybrid)
		if err != nil {
			t.Fatalf("InitOtel with hybrid mode failed: %v", err)
		}
		if shutdown == nil {
			t.Fatal("shutdown function is nil")
		}

		// Verify both pull and push are initialized
		meter := GetMeter("test-meter-hybrid")
		if meter == nil {
			t.Error("GetMeter returned nil for hybrid mode")
		}

		time.Sleep(10 * time.Millisecond)

		// Test Shutdown (may error due to no collector, but shouldn't panic)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdown(ctx) // Ignore error as collector may not be running
	})
}
