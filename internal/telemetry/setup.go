// Package telemetry initializes OpenTelemetry tracing and provides span
// helpers, modeled on the jussi integration pattern. Traces are exported via
// OTLP/HTTP.
package telemetry

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Setup configures the global TracerProvider to export traces via OTLP/HTTP to
// the given endpoint (host:port, e.g. "localhost:4318"). It returns a shutdown
// function that must be called on graceful exit. Errors are non-fatal: callers
// may log and continue without tracing.
//
// Unlike jussi, the endpoint is taken as-is (host:port) — no silent port
// rewriting. Use WithInsecure for local dev; for production TLS omit that by
// passing a config flag (TODO in M-hardening).
func Setup(serviceName, endpoint string, log zerolog.Logger) (func(), error) {
	ctx := context.Background()

	// Resource identifies this service in exported spans.
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
	)
	if err != nil {
		return nil, err
	}

	// OTLP/HTTP exporter. WithInsecure = plaintext, suitable for a local
	// collector on the same network. TODO: add TLS option for production.
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
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

	log.Info().Str("endpoint", endpoint).Str("service", serviceName).Msg("OpenTelemetry tracing initialized")

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("OpenTelemetry tracer shutdown error")
		}
	}
	return shutdown, nil
}
