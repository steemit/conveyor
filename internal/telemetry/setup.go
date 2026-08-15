// Package telemetry initializes OpenTelemetry tracing and provides span
// helpers, modeled on the jussi integration pattern. Traces are exported via
// OTLP/HTTP, with support for OpenObserve-style targets: custom URL paths
// (/api/<org>/v1/traces) and extra headers (Basic Auth), configured through
// telemetry.otlp_path / telemetry.otlp_headers.
package telemetry

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"github.com/steemit/conveyor/internal/config"
)

// Setup configures the global TracerProvider to export traces via OTLP/HTTP.
// It returns a shutdown function that must be called on graceful exit. Errors
// are non-fatal: callers may log and continue without tracing.
//
// Endpoint accepts "host:port" or a full "http(s)://host[:port][/path]" URL;
// in URL form the port defaults to 4318 and the path is used unless OTLPPath
// overrides it.
func Setup(cfg config.TelemetryConfig, log zerolog.Logger) (func(), error) {
	ctx := context.Background()

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	opts := buildExporterOptions(cfg)
	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// W3C TraceContext + Baggage propagation.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Info().
		Str("endpoint", cfg.Endpoint).
		Str("path", cfg.OTLPPath).
		Str("service", cfg.ServiceName).
		Msg("OpenTelemetry tracing initialized")

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("OpenTelemetry tracer shutdown error")
		}
	}
	return shutdown, nil
}

// buildExporterOptions translates the telemetry config into OTLP/HTTP client
// options, normalizing the endpoint (URL → host:port + path) and applying
// the custom URL path and headers OpenObserve requires.
func buildExporterOptions(cfg config.TelemetryConfig) []otlptracehttp.Option {
	endpoint := cfg.Endpoint
	urlPath := cfg.OTLPPath
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		if parsed, err := url.Parse(endpoint); err == nil {
			host := parsed.Host
			if parsed.Port() == "" {
				host += ":4318"
			}
			endpoint = host
			// A URL path contributes only when OTLPPath does not override it.
			if urlPath == "" && parsed.Path != "" && parsed.Path != "/" {
				urlPath = parsed.Path
			}
		}
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // plaintext; suitable for in-VPC collectors
	}
	if urlPath != "" {
		// OpenObserve: /api/<org>/v1/traces instead of the default /v1/traces.
		opts = append(opts, otlptracehttp.WithURLPath(urlPath))
	}
	if len(cfg.OTLPHeaders) > 0 {
		// OpenObserve Basic Auth: Authorization: Basic <base64>.
		opts = append(opts, otlptracehttp.WithHeaders(cfg.OTLPHeaders))
	}
	return opts
}

// buildResource identifies this service in exported spans, folding in
// configured resource attributes (e.g. deployment.environment=dev).
func buildResource(ctx context.Context, cfg config.TelemetryConfig) (*resource.Resource, error) {
	attrs := []resource.Option{
		resource.WithAttributes(semconv.ServiceNameKey.String(cfg.ServiceName)),
	}
	if len(cfg.ResourceAttributes) > 0 {
		kvs := make([]attribute.KeyValue, 0, len(cfg.ResourceAttributes))
		for k, v := range cfg.ResourceAttributes {
			kvs = append(kvs, attribute.String(k, v))
		}
		attrs = append(attrs, resource.WithAttributes(kvs...))
	}
	return resource.New(ctx, attrs...)
}
