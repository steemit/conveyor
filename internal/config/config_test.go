package config

import (
	"testing"
)

// Load reads config/default.toml from a CWD-relative path, so tests must run
// from the repo root.
func chdirRepoRoot(t *testing.T) {
	t.Helper()
	t.Chdir("../..")
}

// TestLoad_TelemetryEnvVars_NoMapUnmarshalError reproduces the pr109 dev
// deployment failure: map-typed telemetry env vars (headers,
// resource_attributes) must be parsed manually, never through viper env
// bindings — viper cannot cast "Key=Value,..." into map[string]string and the
// resulting unmarshal error is fatal at startup:
//
//	unmarshal config: 'telemetry.resource_attributes' expected a map, got 'string'
func TestLoad_TelemetryEnvVars_NoMapUnmarshalError(t *testing.T) {
	chdirRepoRoot(t)
	env := map[string]string{
		"CONVEYOR_TELEMETRY_ENABLED":            "true",
		"CONVEYOR_TELEMETRY_SERVICE_NAME":        "conveyor",
		"CONVEYOR_TELEMETRY_OTLP_ENDPOINT":       "http://10.188.1.50:5080",
		"CONVEYOR_TELEMETRY_OTLP_PATH":           "/api/default/v1/traces",
		"CONVEYOR_TELEMETRY_OTLP_HEADERS":        "Authorization=Basic dGVzdA==, X-Custom=1",
		"CONVEYOR_TELEMETRY_RESOURCE_ATTRIBUTES": "deployment.environment=dev",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load must not fail with map-typed telemetry env vars: %v", err)
	}

	if !cfg.Telemetry.Enabled {
		t.Error("telemetry.enabled not applied")
	}
	if cfg.Telemetry.Endpoint != "http://10.188.1.50:5080" {
		t.Errorf("endpoint: got %q", cfg.Telemetry.Endpoint)
	}
	if cfg.Telemetry.OTLPPath != "/api/default/v1/traces" {
		t.Errorf("otlp_path: got %q", cfg.Telemetry.OTLPPath)
	}
	if got := cfg.Telemetry.OTLPHeaders["Authorization"]; got != "Basic dGVzdA==" {
		t.Errorf("Authorization header: got %q", got)
	}
	if got := cfg.Telemetry.OTLPHeaders["X-Custom"]; got != "1" {
		t.Errorf("X-Custom header: got %q", got)
	}
	if got := cfg.Telemetry.ResourceAttributes["deployment.environment"]; got != "dev" {
		t.Errorf("resource attribute: got %q", got)
	}
}

// TestLoad_CommaSeparatedTrustedProxies covers the other manually parsed env
// var.
func TestLoad_CommaSeparatedTrustedProxies(t *testing.T) {
	chdirRepoRoot(t)
	t.Setenv("TRUSTED_PROXIES", " 172.16.0.0/12 , 10.0.0.0/8 ,")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "172.16.0.0/12" || cfg.TrustedProxies[1] != "10.0.0.0/8" {
		t.Errorf("trusted proxies: got %v", cfg.TrustedProxies)
	}
}

// TestParseKeyValueEnv guards the "Key=Value" parser itself.
func TestParseKeyValueEnv(t *testing.T) {
	if m := parseKeyValueEnv(""); len(m) != 0 {
		t.Errorf("empty: got %v", m)
	}
	if m := parseKeyValueEnv("   "); len(m) != 0 {
		t.Errorf("blank: got %v", m)
	}
	m := parseKeyValueEnv("A=1, B = 2 ,noequals,=3,C=")
	if len(m) != 3 || m["A"] != "1" || m["B"] != "2" || m["C"] != "" {
		t.Errorf("got %v", m)
	}
}
