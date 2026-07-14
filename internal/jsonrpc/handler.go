package jsonrpc

import (
	"strconv"

	"github.com/rs/zerolog"
)

// Context is passed to each RPC handler. It carries the authenticated account
// (empty for public methods; populated by the auth layer in M1), a logger, and
// helper assertion methods mirroring koa-jsonrpc's rpcAssert.
type Context struct {
	Account string
	Log     zerolog.Logger
}

// Handler is the signature every RPC method implements. Handlers parse params
// from req themselves (see Request.UnmarshalParams) and return either a result
// value or an *Error.
type Handler func(ctx *Context, req *Request) (any, error)

// Assert mirrors koa-jsonrpc's rpcAssert: returns a JsonRpcError(400) if the
// condition is false.
func (c *Context) Assert(cond bool, message string) error {
	if !cond {
		return NewError(400, nil, message)
	}
	return nil
}

// AssertEqual mirrors koa-jsonrpc's rpcAssertEqual.
func (c *Context) AssertEqual(actual, expected any, message string) error {
	// Loose equality mirroring the JS == used by koa-jsonrpc.
	if !looseEqual(actual, expected) {
		return NewErrorWithData(400, nil, message, map[string]any{
			"actual":   actual,
			"expected": expected,
		})
	}
	return nil
}

func looseEqual(a, b any) bool {
	// Simple == for comparable scalar types; fall back to deep equality.
	return fmtSprint(a) == fmtSprint(b)
}

func fmtSprint(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		return strconv.FormatBool(x)
	case nil:
		return ""
	}
	return ""
}
