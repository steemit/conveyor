package jsonrpc

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/rs/zerolog"

	"github.com/steemit/conveyor/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// dispatch processes a single parsed request object and returns its response
// (or nil if it is a notification that should not be answered).
func (s *Server) dispatch(ctx context.Context, data json.RawMessage, baseLog zerolog.Logger) *Response {
	// Parse the raw envelope.
	var raw rawRequest
	if err := json.Unmarshal(data, &raw); err != nil {
		e := ErrInvalidRequest(err)
		return &Response{JSONRPC: "2.0", ID: ID{kind: idNull}, Error: e}
	}

	id, idPresent, err := parseID(raw.ID)
	if err != nil || raw.JSONRPC != "2.0" || raw.Method == "" {
		e := ErrInvalidRequest(err)
		// Per spec, if the id can't be determined, use null.
		respID := ID{kind: idNull}
		if err == nil {
			respID = id
		}
		return &Response{JSONRPC: "2.0", ID: respID, Error: e}
	}

	// A notification is a request without an id field (not the same as id:null).
	isNotification := !idPresent

	logCtx := baseLog.With().Str("method", raw.Method).Str("id", id.String()).Logger()

	handler, ok := s.methods[raw.Method]
	if !ok {
		if isNotification {
			return nil
		}
		return &Response{JSONRPC: "2.0", ID: id, Error: ErrMethodNotFound(id)}
	}

	req := &Request{ID: id, Method: raw.Method, Params: raw.Params}

	// Open an internal span for the method dispatch.
	spanCtx, span := telemetry.StartSpan(ctx, "conveyor.process_request",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	_ = spanCtx // child spans within the handler will use this in later milestones
	span.SetAttributes(
		attribute.String("rpc.method", raw.Method),
		attribute.String("rpc.id", id.String()),
	)

	hctx := &Context{Log: logCtx}
	result, err := handler(hctx, req)
	if err != nil {
		e, ok := err.(*Error)
		if !ok {
			e = ErrInternalError(err)
		}
		telemetry.RecordSpanError(span, e)
		if isNotification {
			return nil
		}
		return &Response{JSONRPC: "2.0", ID: id, Error: e}
	}
	telemetry.SetSpanSuccess(span)

	if isNotification {
		return nil
	}
	// koa-jsonrpc: result === undefined -> null. In Go, a nil interface maps to
	// null automatically, but an explicit nil result is marshalled as null.
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

// handleBatch processes a batch of requests concurrently, returning the
// responses for non-notification requests (possibly empty).
func (s *Server) handleBatch(ctx context.Context, items []json.RawMessage, baseLog zerolog.Logger) []*Response {
	spanCtx, span := telemetry.StartSpan(ctx, "conveyor.process_batch",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	responses := make([]*Response, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, d json.RawMessage) {
			defer wg.Done()
			responses[idx] = s.dispatch(spanCtx, d, baseLog)
		}(i, item)
	}
	wg.Wait()

	// Filter out nil (notifications) in place, preserving order.
	out := responses[:0]
	for _, r := range responses {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}
