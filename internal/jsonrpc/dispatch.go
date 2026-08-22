package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/steemit/conveyor/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// maxBatchConcurrency bounds how many batch sub-requests are dispatched at
// once. Without it every item spawns its own unbounded goroutine, so a single
// request can create thousands of concurrent dispatches (audit 2026-08-18
// T-001).
const maxBatchConcurrency = 8

// dispatch processes a single parsed request object and returns its response
// (or nil if it is a notification that should not be answered).
func (s *Server) dispatch(ctx context.Context, data json.RawMessage, baseLog zerolog.Logger, clientIP string) *Response {
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

	entry, ok := s.methods[raw.Method]
	if !ok {
		if isNotification {
			return nil
		}
		return &Response{JSONRPC: "2.0", ID: id, Error: ErrMethodNotFound()}
	}

	// Open an internal span for the method dispatch.
	spanCtx, span := telemetry.StartSpan(ctx, "conveyor.process_request",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()
	span.SetAttributes(
		attribute.String("rpc.method", raw.Method),
		attribute.String("rpc.id", id.String()),
	)

	req := &Request{ID: id, Method: raw.Method, Params: raw.Params, Ctx: spanCtx}
	hctx := &Context{Log: logCtx, IP: clientIP}

	// Authenticated methods: verify __signed before invoking the handler.
	if entry.auth {
		if s.auth == nil {
			e := &Error{Code: InternalError, Message: "auth not configured"}
			if isNotification {
				return nil
			}
			return &Response{JSONRPC: "2.0", ID: id, Error: e}
		}
		if !hasSignedWrapper(req.Params) {
			// koa-jsonrpc's resolveParams rejects a non-__signed params object
			// for authenticated methods with InvalidParams (-32602).
			e := &Error{Code: InvalidParams, Message: "Invalid params: __signed required"}
			if isNotification {
				return nil
			}
			return &Response{JSONRPC: "2.0", ID: id, Error: e}
		}
		decoded, account, authErr := s.auth.Authenticate(spanCtx, req.Method, req.Params)
		if authErr != nil {
			telemetry.RecordSpanError(span, authErr)
			if isNotification {
				return nil
			}
			return &Response{JSONRPC: "2.0", ID: id, Error: authErr}
		}
		req.Params = decoded
		hctx.Account = account
	}

	handler := entry.handler
	result, err := handler(hctx, req)
	if err != nil {
		e, ok := err.(*Error)
		if !ok {
			// A plain error is an unexpected internal failure: log the
			// detail, return a generic message (audit 2026-08-18 T-007).
			logCtx.Error().Err(err).Msg("rpc handler internal error")
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
func (s *Server) handleBatch(ctx context.Context, items []json.RawMessage, baseLog zerolog.Logger, clientIP string) []*Response {
	spanCtx, span := telemetry.StartSpan(ctx, "conveyor.process_batch",
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal))
	defer span.End()

	responses := make([]*Response, len(items))
	sem := make(chan struct{}, maxBatchConcurrency)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, d json.RawMessage) {
			defer wg.Done()
			defer func() { <-sem }()
			// gin.Recovery() only covers the top-level request goroutine; an
			// unrecovered panic in this child goroutine would crash the whole
			// process. Convert it into a single internal-error response
			// instead (audit 2026-08-18 T-009).
			defer func() {
				if r := recover(); r != nil {
					baseLog.Error().Interface("panic", r).Msg("recovered panic in batch dispatch")
					responses[idx] = &Response{
						JSONRPC: "2.0", ID: ID{kind: idNull},
						Error: ErrInternalError(fmt.Errorf("handler panic")),
					}
				}
			}()
			responses[idx] = s.dispatch(spanCtx, d, baseLog, clientIP)
		}(i, item)
	}
	wg.Wait()

	// Filter responses in place, preserving order. koa-jsonrpc's isValidResponse
	// drops a response when it is a notification (no id) AND has no error.
	// Concretely: nil responses (notifications) are dropped, and responses with
	// id:null but successful (no error) are also dropped — only id:null responses
	// that carry an error are kept.
	out := responses[:0]
	for _, r := range responses {
		if r == nil {
			continue
		}
		if !isValidResponse(r) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// isValidResponse mirrors koa-jsonrpc's isValidResponse: a response is valid
// (should be sent to the client) if it has a non-null id, OR it carries an
// error. A notification (no id / null id) with no error is dropped.
func isValidResponse(r *Response) bool {
	return r.ID.kind != idNull || r.Error != nil
}
