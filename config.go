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
	ServiceName              string  `env:"SERVICE_NAME"`
	Version                  string
	BuildTime                string
	LogLevel                 string  `env:"LOG_LEVEL" env-default:"info"`
	OtelEndpoint             string  `env:"OTEL_ENDPOINT" env-default:"localhost:4318"`
	MetricsPort              int     `env:"METRICS_PORT" env-default:"9090"`
	OtelTracingSampleRate    float64 `env:"OTEL_TRACING_SAMPLE_RATE" env-default:"1.0"`
	MetricsMode              string  `env:"METRICS_MODE" env-default:"pull"`
	MetricsPath              string  `env:"METRICS_PATH" env-default:"/metrics"`
	MetricsPushEndpoint      string  `env:"METRICS_PUSH_ENDPOINT"`
	MetricsPushInterval      int     `env:"METRICS_PUSH_INTERVAL" env-default:"30"`
	MetricsProtocol          string  `env:"METRICS_PROTOCOL" env-default:"http"`
}


func (b *BaseConfig) SetMetadata(s, v, t string) {
	if s != "" && strings.TrimSpace(b.ServiceName) == "" {
		b.ServiceName = s
	}
	b.Version = v
	b.BuildTime = t
}

// IsPull returns true if MetricsMode is "pull" or "hybrid"
func (b *BaseConfig) IsPull() bool {
	mode := strings.ToLower(strings.TrimSpace(b.MetricsMode))
	return mode == "pull" || mode == "hybrid"
}

// IsPush returns true if MetricsMode is "push" or "hybrid"
func (b *BaseConfig) IsPush() bool {
	mode := strings.ToLower(strings.TrimSpace(b.MetricsMode))
	return mode == "push" || mode == "hybrid"
}

// IsHybrid returns true if MetricsMode is "hybrid"
func (b *BaseConfig) IsHybrid() bool {
	mode := strings.ToLower(strings.TrimSpace(b.MetricsMode))
	return mode == "hybrid"
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
		ms.SetMetadata(GetServiceName(), GetVersion(), GetBuildTime())
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
		if GetServiceName() != "" && strings.TrimSpace(f.String()) == "" {
			f.SetString(GetServiceName())
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

	// Logic for MetricsMode validation
	mmField := v.FieldByName("MetricsMode")
	if mmField.IsValid() {
		mm := strings.ToLower(strings.TrimSpace(mmField.String()))
		switch mm {
		case "pull", "push", "hybrid":
		default:
			return fmt.Errorf("invalid METRICS_MODE: %s (must be 'pull', 'push', or 'hybrid')", mm)
		}

		// If push mode, MetricsPushEndpoint is required
		if mm == "push" || mm == "hybrid" {
			epField := v.FieldByName("MetricsPushEndpoint")
			if epField.IsValid() && strings.TrimSpace(epField.String()) == "" {
				return fmt.Errorf("METRICS_PUSH_ENDPOINT is required for push/hybrid metrics mode")
			}
		}
	}

	// Logic for MetricsProtocol validation
	mpField := v.FieldByName("MetricsProtocol")
	if mpField.IsValid() {
		mp := strings.ToLower(strings.TrimSpace(mpField.String()))
		switch mp {
		case "http", "grpc":
		default:
			return fmt.Errorf("invalid METRICS_PROTOCOL: %s (must be 'http' or 'grpc')", mp)
		}
	}

	return nil
}
