package observability

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

// --- Configuration ---

type MetadataSetter interface {
	SetMetadata(serviceName, version, buildTime string)
}

type BaseConfig struct {
	ServiceName           string  `env:"SERVICE_NAME"`
	Version               string
	BuildTime             string
	LogLevel              string  `env:"LOG_LEVEL" env-default:"info"`
	OtelEndpoint          string  `env:"OTEL_ENDPOINT" env-default:"localhost:4318"`
	MetricsPort           int     `env:"METRICS_PORT" env-default:"9090"`
	OtelTracingSampleRate float64 `env:"OTEL_TRACING_SAMPLE_RATE" env-default:"1.0"`
}

func (b *BaseConfig) SetMetadata(s, v, t string) {
	if s != "" {
		b.ServiceName = s
	}
	b.Version = v
	b.BuildTime = t
}

func LoadCfg(cfg any) error {
	// 1. Priority: .env > Environment Variables
	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", cfg); err != nil {
			return fmt.Errorf("read .env failed: %w", err)
		}
	} else {
		if err := cleanenv.ReadEnv(cfg); err != nil {
			return fmt.Errorf("read env failed: %w", err)
		}
	}

	// 2. Inject LDFlags if applicable
	if ms, ok := cfg.(MetadataSetter); ok {
		ms.SetMetadata(ServiceName, Version, BuildTime)
	}

	// 3. Post-processing & Validation
	return finalizeAndValidate(cfg)
}

func finalizeAndValidate(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Logic for ServiceName (LDFlags override)
	f := v.FieldByName("ServiceName")
	if f.IsValid() && f.CanSet() {
		if ServiceName != "" {
			f.SetString(ServiceName)
		}
		if strings.TrimSpace(f.String()) == "" {
			return fmt.Errorf("SERVICE_NAME is required")
		}
	}

	// Logic for LogLevel validation
	lvField := v.FieldByName("LogLevel")
	if lvField.IsValid() {
		lv := strings.ToLower(lvField.String())
		switch lv {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("invalid LOG_LEVEL: %s", lv)
		}
	}

	return nil
}
