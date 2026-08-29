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
	Port      string          `mapstructure:"port"`
	Name      string          `mapstructure:"name"`
	RpcNode   string          `mapstructure:"rpc_node"`
	AdminRole string          `mapstructure:"admin_role"`
	Log       []LogStream     `mapstructure:"log"`
	Telemetry TelemetryConfig `mapstructure:"telemetry"`
	Storage   StorageConfig   `mapstructure:"storage"`
	Database  DatabaseConfig  `mapstructure:"database"`
	AccountsRefreshInterval int              `mapstructure:"accounts_refresh_interval"`
	CacheClient             CacheClientConfig `mapstructure:"cacheClient"`
	// TrustedProxies is the list of trusted proxy IPs/CIDRs (the direct
	// upstream, e.g. openresty) whose X-Forwarded-For is honored when deriving
	// the client IP. Empty (default) trusts NO proxy — only the TCP peer is
	// used — which is the safe default. Configure via TRUSTED_PROXIES (comma
	// separated) in production behind a reverse proxy.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
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
	// Endpoint is the OTLP/HTTP target. Accepts host:port (e.g.
	// "localhost:4318") or a full URL ("http://10.188.1.50:5080"); URL form
	// also contributes its path when OTLPPath is unset.
	Endpoint string `mapstructure:"otlp_endpoint"`
	// OTLPPath overrides the export URL path. OpenObserve needs
	// "/api/<org>/v1/traces" instead of the default "/v1/traces".
	OTLPPath string `mapstructure:"otlp_path"`
	// OTLPHeaders carries extra export headers, e.g. OpenObserve Basic Auth:
	// "Authorization=Basic <base64>". Env form is comma-separated
	// "Key=Value" pairs (viper cannot split those into a map itself).
	OTLPHeaders map[string]string `mapstructure:"otlp_headers"`
	// ResourceAttributes adds resource attributes, e.g.
	// "deployment.environment=dev" (comma-separated Key=Value env form).
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
}

// StorageConfig configures the blob store used for drafts and feature-flags.
type StorageConfig struct {
	Type     string `mapstructure:"type"`      // memory | s3
	S3Bucket string `mapstructure:"s3_bucket"` // required when type=s3
}

// DatabaseConfig configures the relational database (user data, tags).
type DatabaseConfig struct {
	Dialect  string `mapstructure:"dialect"`  // sqlite | postgres
	Name     string `mapstructure:"database"` // postgres dbname
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	// SSLMode is the libpq sslmode for the postgres dialect:
	// disable | allow | prefer | require | verify-ca | verify-full.
	// Default "prefer" keeps self-hosted deployments friction-free (encrypt
	// when the server supports TLS, plain otherwise). Deployments carrying
	// PII (steemit production) should set verify-full via DATABASE_SSL_MODE
	// together with DATABASE_SSL_ROOT_CERT (audit 2026-08-18 T-008).
	SSLMode string `mapstructure:"ssl_mode"`
	// SSLRootCert is an optional path to a CA bundle used by verify-ca /
	// verify-full (e.g. the RDS regional bundle vendored at
	// certs/rds-us-east-1-bundle.pem). Ignored by other modes.
	SSLRootCert string `mapstructure:"ssl_root_cert"`
}

// validSSLModes lists the accepted libpq sslmode values.
var validSSLModes = map[string]bool{
	"disable": true, "allow": true, "prefer": true,
	"require": true, "verify-ca": true, "verify-full": true,
}

// CacheClientConfig configures the user-search CachingClient TTL.
type CacheClientConfig struct {
	TTL      int `mapstructure:"ttl"`      // seconds (default 600)
	Interval int `mapstructure:"interval"` // cache cleanup interval seconds (default 60)
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
	v.SetDefault("rpc_node", "https://api.steemit.com")
	v.SetDefault("admin_role", "foo")
	v.SetDefault("accounts_refresh_interval", 600000)
	v.SetDefault("cacheClient.ttl", 600)
	v.SetDefault("cacheClient.interval", 60)
	v.SetDefault("database.ssl_mode", "prefer")

	// Telemetry defaults.
	v.SetDefault("telemetry.enabled", true)
	v.SetDefault("telemetry.service_name", "conveyor")
	v.SetDefault("telemetry.otlp_endpoint", "localhost:4318")

	// Environment variable overrides (explicit binding; avoid AutomaticEnv which
	// mangles underscore keys). Names mirror custom-environment-variables.toml.
	bindEnv(v, "port", "PORT")
	bindEnv(v, "name", "NAME")
	bindEnv(v, "rpc_node", "RPC_NODE")
	bindEnv(v, "admin_role", "ADMIN_ROLE")
	bindEnv(v, "storage.type", "STORAGE_TYPE")
	bindEnv(v, "storage.s3_bucket", "S3_BUCKET")
	bindEnv(v, "database.database", "DATABASE_NAME")
	bindEnv(v, "database.host", "DATABASE_HOST")
	bindEnv(v, "database.port", "DATABASE_PORT")
	bindEnv(v, "database.username", "DATABASE_USERNAME")
	bindEnv(v, "database.password", "DATABASE_PASSWORD")
	bindEnv(v, "database.dialect", "DATABASE_DIALECT")
	bindEnv(v, "database.ssl_mode", "DATABASE_SSL_MODE")
	bindEnv(v, "database.ssl_root_cert", "DATABASE_SSL_ROOT_CERT")
	bindEnv(v, "telemetry.enabled", "CONVEYOR_TELEMETRY_ENABLED")
	bindEnv(v, "telemetry.service_name", "CONVEYOR_TELEMETRY_SERVICE_NAME")
	bindEnv(v, "telemetry.otlp_endpoint", "CONVEYOR_TELEMETRY_OTLP_ENDPOINT")
	bindEnv(v, "telemetry.otlp_path", "CONVEYOR_TELEMETRY_OTLP_PATH")
	// NOTE: NO viper env binding for map-typed fields (otlp_headers,
	// resource_attributes) — viper cannot cast a "Key=Value,..." env string
	// into map[string]string and the unmarshal error is FATAL at startup
	// ("expected a map, got 'string'", observed on the pr109 dev deploy).
	// They are parsed manually below (same approach as jussi's fix for
	// JUSSI_TELEMETRY_OTLP_HEADERS, commit 758133f).
	bindEnv(v, "trusted_proxies", "TRUSTED_PROXIES")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	// Fail fast on a mistyped sslmode instead of surfacing as an opaque
	// libpq connection error at first DB use.
	if cfg.Database.SSLMode != "" && !validSSLModes[cfg.Database.SSLMode] {
		return nil, fmt.Errorf("database.ssl_mode: invalid value %q (valid: disable, allow, prefer, require, verify-ca, verify-full)", cfg.Database.SSLMode)
	}
	// viper binds a comma-separated env var as a single string; split it into
	// a slice so []string unmarshals cleanly.
	if raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXIES")); raw != "" {
		parts := strings.Split(raw, ",")
		cfg.TrustedProxies = make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				cfg.TrustedProxies = append(cfg.TrustedProxies, t)
			}
		}
	}
	// Comma-separated "Key=Value" env vars for map-typed telemetry fields
	// (viper cannot split those into maps itself). Mirrors jussi's
	// JUSSI_TELEMETRY_OTLP_HEADERS handling.
	if m := parseKeyValueEnv(os.Getenv("CONVEYOR_TELEMETRY_OTLP_HEADERS")); len(m) > 0 {
		cfg.Telemetry.OTLPHeaders = m
	}
	if m := parseKeyValueEnv(os.Getenv("CONVEYOR_TELEMETRY_RESOURCE_ATTRIBUTES")); len(m) > 0 {
		cfg.Telemetry.ResourceAttributes = m
	}
	return &cfg, nil
}

// parseKeyValueEnv parses "Key=Value,Key2=Value2" into a map. Entries without
// '=' are skipped; keys and values are trimmed.
func parseKeyValueEnv(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	m := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key != "" {
			m[key] = strings.TrimSpace(parts[1])
		}
	}
	return m
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
