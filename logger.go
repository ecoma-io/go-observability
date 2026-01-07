package observability

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// --- Logging ---

type Logger struct {
	*zap.SugaredLogger
}

func NewLogger(cfg *BaseConfig) *Logger {
	level := zapcore.InfoLevel
	service := "unknown"
	version := "unknown"

	if cfg != nil {
		if parsed, err := zapcore.ParseLevel(cfg.LogLevel); err == nil {
			level = parsed
		}
		service = cfg.ServiceName
		version = cfg.Version
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.TimeKey = "timestamp"

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(os.Stdout),
		level,
	)

	l := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	l = l.With(zap.String("service", service), zap.String("version", version))

	return &Logger{SugaredLogger: l.Sugar()}
}

// Helper methods for logging
func (l *Logger) Info(msg string, args ...any)  { l.Infow(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.Errorw(msg, args...) }
func (l *Logger) Debug(msg string, args ...any) { l.Debugw(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.Warnw(msg, args...) }
func (l *Logger) Fatal(msg string, args ...any) { l.Fatalw(msg, args...) }
func (l *Logger) Sync()                         { _ = l.SugaredLogger.Sync() }
