// Package config loads conveyor configuration from TOML files and environment
// variables, mirroring the layout of the original node-config based TS service.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config is the parsed application configuration.
type Config struct {
	Port      string        `mapstructure:"port"`
	Name      string        `mapstructure:"name"`
	NumWorkers int          `mapstructure:"num_workers"`
	Log       []LogStream   `mapstructure:"log"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
}

// LogStream describes a single logging output stream.
type LogStream struct {
	Level string `mapstructure:"level"`
	Out   string `mapstructure:"out"` // stdout | stderr | file path
}

// TelemetryConfig configures OpenTelemetry tracing export.
type TelemetryConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ServiceName string `mapstructure:"service_name"`
	Endpoint    string `mapstructure:"otlp_endpoint"` // host:port, e.g. localhost:4318
}

// Load reads config/default.toml, then the environment-specific file based on
// NODE_ENV (production/test), then environment variable overrides.
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("default")
	v.SetConfigType("toml")
	v.AddConfigPath("config")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read default config: %w", err)
	}

	// Overlay environment-specific config (production.toml / test.toml).
	env := envOrDefault("NODE_ENV", "")
	if env != "" {
		ov := viper.New()
		ov.SetConfigName(env)
		ov.SetConfigType("toml")
		ov.AddConfigPath("config")
		if err := ov.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, fmt.Errorf("read %s config: %w", env, err)
			}
		} else {
			if err := v.MergeConfigMap(ov.AllSettings()); err != nil {
				return nil, fmt.Errorf("merge %s config: %w", env, err)
			}
		}
	}

	// Core defaults (also present in default.toml, but set explicitly so the
	// service still boots if that file is missing or partially overridden).
	v.SetDefault("port", "8090")
	v.SetDefault("name", "conveyor")

	// Telemetry defaults.
	v.SetDefault("telemetry.enabled", true)
	v.SetDefault("telemetry.service_name", "conveyor")
	v.SetDefault("telemetry.otlp_endpoint", "localhost:4318")

	// Environment variable overrides (explicit binding; avoid AutomaticEnv which
	// mangles underscore keys). Names mirror custom-environment-variables.toml.
	bindEnv(v, "port", "PORT")
	bindEnv(v, "name", "NAME")
	bindEnv(v, "num_workers", "NUM_WORKERS")
	bindEnv(v, "telemetry.enabled", "CONVEYOR_TELEMETRY_ENABLED")
	bindEnv(v, "telemetry.service_name", "CONVEYOR_TELEMETRY_SERVICE_NAME")
	bindEnv(v, "telemetry.otlp_endpoint", "CONVEYOR_TELEMETRY_OTLP_ENDPOINT")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &cfg, nil
}

func bindEnv(v *viper.Viper, key, env string) {
	_ = v.BindEnv(key, env)
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
