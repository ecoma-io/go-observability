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
}
