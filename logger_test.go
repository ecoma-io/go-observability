package observability

import (
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Run("Default Config", func(t *testing.T) {
		cfg := &BaseConfig{
			ServiceName: "test-logger",
			LogLevel:    "info",
		}
		l := NewLogger(cfg)
		if l == nil {
			t.Fatal("NewLogger returned nil")
		}
		// Since l.SugaredLogger is embedded, we can't easily check internal level
		// without using unsafe or verifying behavior.
		// But verify methods don't panic
		l.Info("test info message")
	})

	t.Run("Nil Config", func(t *testing.T) {
		l := NewLogger(nil)
		if l == nil {
			t.Fatal("NewLogger(nil) returned nil")
		}
		l.Info("test info message with nil config")
	})

	t.Run("Invalid Log Level Falls back to Info", func(t *testing.T) {
		// This unit test is tricky because NewLogger logic:
		// if parsed, err := zapcore.ParseLevel(cfg.LogLevel); err == nil { level = parsed }
		// if err != nil, it keeps default 'InfoLevel'.
		// We want to confirm it doesn't crash.
		cfg := &BaseConfig{
			ServiceName: "test-logger",
			LogLevel:    "invalid", // Should trigger error in zapcore.ParseLevel
		}
		l := NewLogger(cfg)
		if l == nil {
			t.Fatal("NewLogger returned nil")
		}
		l.Info("should be info level")
	})
}

// Check if Logger implements expected methods
func TestLoggerMethods(t *testing.T) {
	cfg := &BaseConfig{
		ServiceName: "test-methods",
		LogLevel:    "debug",
	}
	l := NewLogger(cfg)

	// Just calling them to Ensure no panics
	l.Debug("debug msg", "key", "val")
	l.Info("info msg", "key", "val")
	l.Warn("warn msg", "key", "val")
	l.Error("error msg", "key", "val")
	// l.Fatal will exit the program, so we skip it or mock os.Exit if possible (hard with Zap)
}
