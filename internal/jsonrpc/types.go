// Package jsonrpc implements a JSON-RPC 2.0 server that mirrors the request/
// response envelope and error semantics of @steemit/koa-jsonrpc, so the Go
// conveyor is a drop-in replacement for the original TS service.
package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// Standard JSON-RPC 2.0 error codes (matching koa-jsonrpc's JsonRpcErrorCode).
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// ID is a JSON-RPC request id: null, string, or integer (safe-integer).
// A missing id (notification) is represented as a nil *ID (distinct from a
// present-but-null id).
type ID struct {
	// kind identifies how the id appeared on the wire.
	kind idKind
	str  string
	num  int64
}

type idKind int

const (
	idNull idKind = iota
	idString
	idNumber
)

// String returns a human-readable representation for logging.
func (i *ID) String() string {
	if i == nil {
		return "<none>" // notification: no id field at all
	}
	switch i.kind {
	case idNull:
		return "null"
	case idString:
		return fmt.Sprintf("%q", i.str)
	case idNumber:
		return fmt.Sprintf("%d", i.num)
	}
	return "?"
}

// MarshalJSON encodes the id back to JSON for responses.
func (i ID) MarshalJSON() ([]byte, error) {
	switch i.kind {
	case idNull:
		return []byte("null"), nil
	case idString:
		return json.Marshal(i.str)
	case idNumber:
		return json.Marshal(i.num)
	}
	return []byte("null"), nil
}

// rawRequest is the wire-level incoming request.
type rawRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// Request is a parsed JSON-RPC request handed to handlers.
type Request struct {
	ID     ID
	Method string
	Params json.RawMessage // may be empty if omitted
	Ctx    context.Context // the HTTP request context (for store/db calls)
}

// HasParams reports whether the request included a params field.
func (r *Request) HasParams() bool { return len(r.Params) > 0 }

// UnmarshalParams decodes the request params into the target. If the request
// has no params field, the target is left untouched and nil is returned.
func (r *Request) UnmarshalParams(target any) error {
	if !r.HasParams() {
		return nil
	}
	if err := json.Unmarshal(r.Params, target); err != nil {
		return ErrInvalidParams(err)
	}
	return nil
}

// Response is a JSON-RPC 2.0 response envelope.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      ID     `json:"id"`
	Result  any    `json:"-"` // serialized via MarshalJSON to always include result:null
	Error   *Error `json:"-"`
}

// MarshalJSON serializes a Response. A successful response always includes a
// "result" field (null when nil); an error response includes "error". This
// matches koa-jsonrpc, which writes result:null rather than omitting it.
func (r Response) MarshalJSON() ([]byte, error) {
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string `json:"jsonrpc"`
			ID      ID     `json:"id"`
			Error   *Error `json:"error"`
		}{r.JSONRPC, r.ID, r.Error})
	}
	return json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      ID     `json:"id"`
		Result  any    `json:"result"`
	}{r.JSONRPC, r.ID, r.Result})
}

// parseID decodes a raw JSON id value, enforcing the JSON-RPC 2.0 id rules
// (null, string, or safe integer). present=false means the id field was absent.
func parseID(raw json.RawMessage) (id ID, present bool, err error) {
	if len(raw) == 0 {
		return ID{kind: idNull}, false, nil
	}
	// Check for explicit JSON null first — Go's json.Unmarshal treats null as
	// "leave unchanged" for string/float targets, so checking null last would
	// misparse id:null as an empty-string id.
	if string(raw) == "null" {
		return ID{kind: idNull}, true, nil
	}
	// Try string.
	var s string
	if e := json.Unmarshal(raw, &s); e == nil {
		return ID{kind: idString, str: s}, true, nil
	}
	// Try number.
	var f float64
	if e := json.Unmarshal(raw, &f); e == nil {
		if !isSafeInteger(f) {
			return ID{kind: idNull}, true, fmt.Errorf("invalid id")
		}
		return ID{kind: idNumber, num: int64(f)}, true, nil
	}
	return ID{kind: idNull}, true, fmt.Errorf("invalid id")
}

func isSafeInteger(f float64) bool {
	// JSON-RPC id must be a "safe integer" (|f| <= 2^53-1) with no fraction.
	return f == math.Trunc(f) && f >= -(1<<53) && f <= (1<<53)
}
