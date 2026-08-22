package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func testLog() zerolog.Logger {
	return zerolog.Nop()
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// dispatchSingle is a test helper that parses a raw JSON object and dispatches it.
func dispatchSingle(t *testing.T, s *Server, raw string) *Response {
	t.Helper()
	resp := s.dispatch(context.Background(), json.RawMessage(raw), testLog(), "")
	return resp
}

func TestDispatch_ResultValue(t *testing.T) {
	s := NewServer()
	s.Register("echo", func(ctx *Context, req *Request) (any, error) {
		var p struct{ Msg string `json:"msg"` }
		_ = req.UnmarshalParams(&p)
		return p.Msg, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1,"method":"echo","params":{"msg":"hi"}}`)
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.Result != "hi" {
		t.Fatalf("expected result 'hi', got %v", resp.Result)
	}
	// ID must be echoed.
	if resp.ID.kind != idNumber || resp.ID.num != 1 {
		t.Fatalf("expected id=1, got %s", resp.ID.String())
	}
}

func TestDispatch_ResultNullWhenNil(t *testing.T) {
	s := NewServer()
	s.Register("noop", func(ctx *Context, req *Request) (any, error) {
		return nil, nil
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":5,"method":"noop"}`)
	if resp == nil {
		t.Fatal("expected response")
	}
	// Serialize to JSON; result should be null (not omitted).
	b, _ := json.Marshal(resp)
	if !strings.Contains(string(b), `"result":null`) {
		t.Fatalf("expected result:null in JSON, got: %s", string(b))
	}
}

func TestDispatch_MethodNotFound(t *testing.T) {
	s := NewServer()
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":2,"method":"ghost"}`)
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Error == nil || resp.Error.Code != MethodNotFound {
		t.Fatalf("expected MethodNotFound, got: %+v", resp.Error)
	}
}

func TestDispatch_InvalidRequest_BadJSONRPC(t *testing.T) {
	s := NewServer()
	resp := dispatchSingle(t, s, `{"jsonrpc":"1.0","id":1,"method":"x"}`)
	if resp.Error == nil || resp.Error.Code != InvalidRequest {
		t.Fatalf("expected InvalidRequest, got %+v", resp.Error)
	}
}

func TestDispatch_InvalidRequest_BadID(t *testing.T) {
	s := NewServer()
	// Non-integer, non-string, non-null id (float with fraction is not safe-int).
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":1.5,"method":"x"}`)
	if resp.Error == nil || resp.Error.Code != InvalidRequest {
		t.Fatalf("expected InvalidRequest for bad id, got %+v", resp.Error)
	}
}

func TestDispatch_Notification_NoResponse(t *testing.T) {
	s := NewServer()
	s.Register("fire", func(ctx *Context, req *Request) (any, error) {
		return "ignored", nil
	})
	// No "id" field -> notification -> nil response.
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","method":"fire","params":{}}`)
	if resp != nil {
		t.Fatalf("notification should return nil, got %+v", resp)
	}
}

func TestDispatch_Notification_ErrorStillNoResponse(t *testing.T) {
	s := NewServer()
	s.Register("boom", func(ctx *Context, req *Request) (any, error) {
		return nil, NewError(500, nil, "kaboom")
	})
	// Notification that errors internally should still not produce a response.
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","method":"boom"}`)
	if resp != nil {
		t.Fatalf("notification error should return nil, got %+v", resp)
	}
}

func TestDispatch_HandlerError_WrappedAsInternal(t *testing.T) {
	s := NewServer()
	s.Register("err", func(ctx *Context, req *Request) (any, error) {
		return nil, &plainError{"disk full"}
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":7,"method":"err"}`)
	if resp.Error == nil || resp.Error.Code != InternalError {
		t.Fatalf("expected InternalError, got %+v", resp.Error)
	}
	// The wire message must be generic; the cause only shows up in Error()
	// (server-side logs), never in Message (audit 2026-08-18 T-007).
	if resp.Error.Message != "Internal error" {
		t.Fatalf("expected generic message, got: %s", resp.Error.Message)
	}
	if !strings.Contains(resp.Error.Error(), "disk full") {
		t.Fatalf("expected cause preserved in Error(), got: %s", resp.Error.Error())
	}
}

// TestError_CauseNotOnWire verifies that NewError with a cause keeps the
// message clean for clients while Error() retains the detail for logs, and
// that JSON marshalling never includes the cause (audit 2026-08-18 T-007).
func TestError_CauseNotOnWire(t *testing.T) {
	e := NewError(InternalError, fmt.Errorf("failed to GetOrderBook: sql: no rows"), "Internal error")
	if e.Message != "Internal error" {
		t.Fatalf("message must stay generic, got: %s", e.Message)
	}
	if !strings.Contains(e.Error(), "GetOrderBook") {
		t.Fatalf("Error() must retain cause detail, got: %s", e.Error())
	}
	if err := errors.Unwrap(e); err == nil || !strings.Contains(err.Error(), "GetOrderBook") {
		t.Fatalf("Unwrap must expose the cause, got: %v", err)
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "GetOrderBook") {
		t.Fatalf("cause leaked into JSON wire format: %s", string(b))
	}
}

func TestDispatch_JSONRPCError_PassedThrough(t *testing.T) {
	s := NewServer()
	s.Register("ce", func(ctx *Context, req *Request) (any, error) {
		return nil, NewError(420, nil, "No such tag")
	})
	resp := dispatchSingle(t, s, `{"jsonrpc":"2.0","id":9,"method":"ce"}`)
	if resp.Error == nil || resp.Error.Code != 420 {
		t.Fatalf("expected code 420, got %+v", resp.Error)
	}
}

// plainError is a non-*Error error to test InternalError wrapping.
type plainError struct{ msg string }

func (e *plainError) Error() string { return e.msg }

func TestBatch_Concurrent(t *testing.T) {
	s := NewServer()
	s.Register("echo", func(ctx *Context, req *Request) (any, error) {
		var p struct{ V int `json:"v"` }
		_ = req.UnmarshalParams(&p)
		return p.V, nil
	})

	items := []json.RawMessage{
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "echo", "params": map[string]any{"v": 10}}),
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "echo", "params": map[string]any{"v": 20}}),
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "method": "echo", "params": map[string]any{"v": 99}}), // notification
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "ghost"}),
	}
	responses := s.handleBatch(context.Background(), items, testLog(), "")
	// Notification filtered out; 3 responses remain.
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses (notification filtered), got %d", len(responses))
	}
}

func TestBatch_AllNotifications_EmptyResult(t *testing.T) {
	s := NewServer()
	s.Register("fire", func(ctx *Context, req *Request) (any, error) { return nil, nil })
	items := []json.RawMessage{
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "method": "fire"}),
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "method": "fire"}),
	}
	responses := s.handleBatch(context.Background(), items, testLog(), "")
	if len(responses) != 0 {
		t.Fatalf("expected 0 responses, got %d", len(responses))
	}
}

// TestBatch_IDNullSuccessFiltered verifies that a request with id:null that
// succeeds is filtered out of batch responses (matching koa-jsonrpc
// isValidResponse), while id:null that errors is kept.
func TestBatch_IDNullSuccessFiltered(t *testing.T) {
	s := NewServer()
	s.Register("ok", func(ctx *Context, req *Request) (any, error) { return "data", nil })
	s.Register("boom", func(ctx *Context, req *Request) (any, error) {
		return nil, NewError(500, nil, "x")
	})
	items := []json.RawMessage{
		// id:null + success -> filtered out by isValidResponse.
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": nil, "method": "ok"}),
		// id:null + error -> kept (error responses are always valid).
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": nil, "method": "boom"}),
		// normal id + success -> kept.
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ok"}),
	}
	responses := s.handleBatch(context.Background(), items, testLog(), "")
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses (id:null success filtered), got %d", len(responses))
	}
	// First kept response is the id:null error, second is id:1 success.
	if responses[0].Error == nil {
		t.Fatalf("expected first response to be the error")
	}
	if responses[1].Result != "data" {
		t.Fatalf("expected second response result 'data', got %v", responses[1].Result)
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	s := NewServer()
	s.Register("x", func(ctx *Context, req *Request) (any, error) { return nil, nil })
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	s.Register("x", func(ctx *Context, req *Request) (any, error) { return nil, nil })
}

// TestBatch_PanicRecovered verifies that a panicking handler inside a batch
// is converted into a single internal-error response instead of crashing the
// process: gin.Recovery() only covers the top-level request goroutine, so
// without the explicit recover in handleBatch the panic would take down the
// whole server (audit 2026-08-18 T-009).
func TestBatch_PanicRecovered(t *testing.T) {
	s := NewServer()
	s.Register("panic", func(ctx *Context, req *Request) (any, error) {
		panic("boom")
	})
	s.Register("ok", func(ctx *Context, req *Request) (any, error) { return "fine", nil })
	items := []json.RawMessage{
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ok"}),
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "panic"}),
		mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "ok"}),
	}
	responses := s.handleBatch(context.Background(), items, testLog(), "")
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	// responses[0] and [2] succeed; responses[1] is the internal error.
	if responses[0].Error != nil || responses[0].Result != "fine" {
		t.Fatalf("unexpected first response: %+v", responses[0])
	}
	if responses[1].Error == nil || responses[1].Error.Code != InternalError {
		t.Fatalf("expected InternalError for panicking item, got %+v", responses[1])
	}
	if responses[2].Error != nil || responses[2].Result != "fine" {
		t.Fatalf("unexpected last response: %+v", responses[2])
	}
}

// TestBatch_ConcurrencyBounded verifies that handleBatch never runs more than
// maxBatchConcurrency sub-requests at once (audit 2026-08-18 T-001).
func TestBatch_ConcurrencyBounded(t *testing.T) {
	s := NewServer()
	var mu sync.Mutex
	var cur, max int
	s.Register("slow", func(ctx *Context, req *Request) (any, error) {
		mu.Lock()
		cur++
		if cur > max {
			max = cur
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		cur--
		mu.Unlock()
		return 1, nil
	})

	const n = maxBatchItems // biggest batch the middleware accepts
	items := make([]json.RawMessage, n)
	for i := 0; i < n; i++ {
		items[i] = mustMarshal(t, map[string]any{"jsonrpc": "2.0", "id": i, "method": "slow"})
	}
	responses := s.handleBatch(context.Background(), items, testLog(), "")
	if len(responses) != n {
		t.Fatalf("expected %d responses, got %d", n, len(responses))
	}
	if max > maxBatchConcurrency {
		t.Fatalf("observed %d concurrent dispatches, want <= %d", max, maxBatchConcurrency)
	}
}
