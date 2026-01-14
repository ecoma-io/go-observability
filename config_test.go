package observability

import (
	"os"
	"testing"
)

func TestLoadCfg(t *testing.T) {
	// Backup original env vars and restore after test
	originalServiceName := os.Getenv("SERVICE_NAME")
	originalLogLevel := os.Getenv("LOG_LEVEL")

	// Renaming .env to ensure tests run against Environment Variables, not local config
	// This helps isolation and verifies the ReadEnv path.
	if _, err := os.Stat(".env"); err == nil {
		if err := os.Rename(".env", ".env.testbak"); err != nil {
			t.Fatalf("Failed to rename .env to .env.testbak: %v", err)
		}
		defer func() {
			if err := os.Rename(".env.testbak", ".env"); err != nil {
				t.Fatalf("Failed to restore .env from .env.testbak: %v", err)
			}
		}()
	}

	defer func() {
		_ = os.Setenv("SERVICE_NAME", originalServiceName)
		_ = os.Setenv("LOG_LEVEL", originalLogLevel)
	}()

	t.Run("Success with Env Vars", func(t *testing.T) {
		_ = os.Setenv("SERVICE_NAME", "test-service")
		_ = os.Setenv("LOG_LEVEL", "debug")
		_ = os.Setenv("METRICS_PORT", "9091")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.ServiceName != "test-service" {
			t.Errorf("Expected ServiceName 'test-service', got '%s'", cfg.ServiceName)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("Expected LogLevel 'debug', got '%s'", cfg.LogLevel)
		}
		if cfg.MetricsPort != 9091 {
			t.Errorf("Expected MetricsPort 9091, got %d", cfg.MetricsPort)
		}
	})

	t.Run("Success with Defaults", func(t *testing.T) {
		_ = os.Unsetenv("SERVICE_NAME")
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Unsetenv("METRICS_PORT")

		// Manually set ServiceName via global var or it will fail validation if ServiceName global is empty
		// In test, ServiceName global is likely empty.
		// LoadCfg uses 'cleanenv' which won't auto-fill ServiceName unless env var exists.
		// finalizeAndValidate checks if ServiceName is set.
		// Let's set the global ServiceName to simulate LDFlags if we want to test that path,
		// OR set env var only for ServiceName.
		
		// Case: ServiceName provided by env, others default
		_ = os.Setenv("SERVICE_NAME", "default-checker")
		
		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.LogLevel != "info" { // Default from struct tag
			t.Errorf("Expected default LogLevel 'info', got '%s'", cfg.LogLevel)
		}
		if cfg.MetricsPort != 9090 { // Default from struct tag
			t.Errorf("Expected default MetricsPort 9090, got %d", cfg.MetricsPort)
		}
	})

	t.Run("Validation Failure on Missing ServiceName", func(t *testing.T) {
		_ = os.Unsetenv("SERVICE_NAME")
		ServiceName = "" // Ensure global is empty

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err == nil {
			t.Error("Expected LoadCfg to fail due to missing SERVICE_NAME, but it succeeded")
		}
	})

	t.Run("Validation Failure on Invalid LogLevel", func(t *testing.T) {
		_ = os.Setenv("SERVICE_NAME", "valid-service")
		_ = os.Setenv("LOG_LEVEL", "invalid-level")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err == nil {
			t.Error("Expected LoadCfg to fail due to invalid LOG_LEVEL, but it succeeded")
		}
	})

	t.Run("Metrics Mode Configuration", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "metrics-test-service")
		_ = os.Setenv("METRICS_MODE", "pull")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.MetricsMode != "pull" {
			t.Errorf("Expected MetricsMode 'pull', got '%s'", cfg.MetricsMode)
		}
		if !cfg.IsPull() {
			t.Error("Expected IsPull() to return true for 'pull' mode")
		}
		if cfg.IsPush() {
			t.Error("Expected IsPush() to return false for 'pull' mode")
		}
	})

	t.Run("Push Metrics Mode Validation", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "push-test-service")
		_ = os.Setenv("METRICS_MODE", "push")
		_ = os.Unsetenv("METRICS_PUSH_ENDPOINT")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err == nil {
			t.Error("Expected LoadCfg to fail for push mode without METRICS_PUSH_ENDPOINT")
		}
	})

	t.Run("Hybrid Metrics Mode", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "hybrid-test-service")
		_ = os.Setenv("METRICS_MODE", "hybrid")
		_ = os.Setenv("METRICS_PUSH_ENDPOINT", "localhost:4318")
		_ = os.Setenv("METRICS_PUSH_INTERVAL", "60")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.MetricsMode != "hybrid" {
			t.Errorf("Expected MetricsMode 'hybrid', got '%s'", cfg.MetricsMode)
		}
		if !cfg.IsPull() {
			t.Error("Expected IsPull() to return true for hybrid mode")
		}
		if !cfg.IsPush() {
			t.Error("Expected IsPush() to return true for hybrid mode")
		}
		if !cfg.IsHybrid() {
			t.Error("Expected IsHybrid() to return true for hybrid mode")
		}
		if cfg.MetricsPushInterval != 60 {
			t.Errorf("Expected MetricsPushInterval 60, got %d", cfg.MetricsPushInterval)
		}
	})

	t.Run("Invalid Metrics Mode", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "invalid-mode-service")
		_ = os.Setenv("METRICS_MODE", "invalid-mode")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err == nil {
			t.Error("Expected LoadCfg to fail due to invalid METRICS_MODE")
		}
	})

	t.Run("Metrics Protocol - HTTP", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "http-protocol-service")
		_ = os.Setenv("METRICS_MODE", "push")
		_ = os.Setenv("METRICS_PUSH_ENDPOINT", "localhost:4318")
		_ = os.Setenv("METRICS_PROTOCOL", "http")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.MetricsProtocol != "http" {
			t.Errorf("Expected MetricsProtocol 'http', got '%s'", cfg.MetricsProtocol)
		}
	})

	t.Run("Metrics Protocol - gRPC", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "grpc-protocol-service")
		_ = os.Setenv("METRICS_MODE", "push")
		_ = os.Setenv("METRICS_PUSH_ENDPOINT", "localhost:4317")
		_ = os.Setenv("METRICS_PROTOCOL", "grpc")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		if cfg.MetricsProtocol != "grpc" {
			t.Errorf("Expected MetricsProtocol 'grpc', got '%s'", cfg.MetricsProtocol)
		}
	})

	t.Run("Invalid Metrics Protocol", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "invalid-protocol-service")
		_ = os.Setenv("METRICS_MODE", "push")
		_ = os.Setenv("METRICS_PUSH_ENDPOINT", "localhost:4318")
		_ = os.Setenv("METRICS_PROTOCOL", "invalid-protocol")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err == nil {
			t.Error("Expected LoadCfg to fail due to invalid METRICS_PROTOCOL")
		}
	})

	t.Run("Default Metrics Protocol", func(t *testing.T) {
		_ = os.Unsetenv("LOG_LEVEL")
		_ = os.Setenv("SERVICE_NAME", "default-protocol-service")
		_ = os.Setenv("METRICS_MODE", "push")
		_ = os.Setenv("METRICS_PUSH_ENDPOINT", "localhost:4318")
		_ = os.Unsetenv("METRICS_PROTOCOL")

		var cfg BaseConfig
		err := LoadCfg(&cfg)
		if err != nil {
			t.Fatalf("LoadCfg failed: %v", err)
		}

		// Default should be "http"
		if cfg.MetricsProtocol != "http" {
			t.Errorf("Expected default MetricsProtocol 'http', got '%s'", cfg.MetricsProtocol)
		}
	})
}
