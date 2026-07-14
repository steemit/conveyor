package jsonrpc

import (
	"encoding/json"
	"fmt"
)

// Error is a JSON-RPC 2.0 error object. When serialized, it produces
// {"code":N,"message":"...","data":{...}} — the data field is omitted when nil,
// matching koa-jsonrpc's JsonRpcError.toJSON().
type Error struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// NewError builds an Error with an optional cause. When cause is non-nil the
// message is joined as "<message>: <cause>" matching VError's convention in
// koa-jsonrpc (e.g. "Parse error: unexpected token").
func NewError(code int, cause error, message string) *Error {
	msg := message
	if cause != nil {
		msg = fmt.Sprintf("%s: %s", message, cause)
	}
	return &Error{Code: code, Message: msg}
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

func ErrMethodNotFound(id ID) *Error {
	return &Error{Code: MethodNotFound, Message: "Method not found"}
}

func ErrInvalidParams(cause error) *Error {
	return NewError(InvalidParams, cause, "Invalid params")
}

func ErrInternalError(cause error) *Error {
	return NewError(InternalError, cause, "Internal error")
}
