package jsonrpc

import (
	"encoding/json"
	"fmt"
)

// Error is a JSON-RPC 2.0 error object. When serialized, it produces
// {"code":N,"message":"...","data":{...}} — the data field is omitted when nil,
// matching koa-jsonrpc's JsonRpcError.toJSON().
//
// Message is the only text sent to the client, so it must stay free of
// internal details (Go types, DB errors, upstream method names). The optional
// cause is kept off the wire and only surfaces in Error() — i.e. in
// server-side logs and telemetry spans (audit 2026-08-18 T-007).
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
	cause   error
}

// Error implements the error interface. It appends the cause (if any) for
// server-side logging; the JSON wire format carries Message alone.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap exposes the cause to errors.Is/As.
func (e *Error) Unwrap() error { return e.cause }

// NewError builds an Error with an optional cause. The cause is NOT included
// in Message (it would leak internal details to unauthenticated callers);
// it is preserved for logging via Error().
func NewError(code int, cause error, message string) *Error {
	return &Error{Code: code, Message: message, cause: cause}
}

// NewErrorWithData is like NewError but attaches a JSON data payload.
func NewErrorWithData(code int, cause error, message string, data any) *Error {
	e := NewError(code, cause, message)
	if data != nil {
		b, err := json.Marshal(data)
		if err == nil {
			e.Data = b
		}
	}
	return e
}

// Convenience constructors for the standard error codes.

func ErrParseError(cause error) *Error {
	return NewError(ParseError, cause, "Parse error")
}

func ErrInvalidRequest(cause error) *Error {
	return NewError(InvalidRequest, cause, "Invalid Request")
}

func ErrMethodNotFound() *Error {
	return &Error{Code: MethodNotFound, Message: "Method not found"}
}

func ErrInvalidParams(cause error) *Error {
	return NewError(InvalidParams, cause, "Invalid params")
}

func ErrInternalError(cause error) *Error {
	return NewError(InternalError, cause, "Internal error")
}
