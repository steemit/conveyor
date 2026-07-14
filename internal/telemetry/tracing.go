package telemetry

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName is the logical name of this service's tracer.
const tracerName = "conveyor"

// StartSpan creates a child span of the current context. Always pair with
// `defer span.End()`.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, opts...)
}

// AddSpanAttributes sets string attributes on a span.
func AddSpanAttributes(span trace.Span, attrs map[string]string) {
	kv := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		kv = append(kv, attribute.String(k, v))
	}
	span.SetAttributes(kv...)
}

// RecordSpanError records an error on the span and marks its status as Error.
func RecordSpanError(span trace.Span, err error) {
	if err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanSuccess marks the span status as Ok.
func SetSpanSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "Success")
}

// RecordSpanParams serializes the params object as a span event named
// "request.params", making it visible in collectors/Jaeger logs.
func RecordSpanParams(span trace.Span, params any) {
	if params == nil {
		return
	}
	b, err := json.Marshal(params)
	if err != nil {
		return
	}
	span.AddEvent("request.params", trace.WithAttributes(
		attribute.String("params", string(b)),
	))
}
