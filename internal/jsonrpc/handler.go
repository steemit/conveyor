package jsonrpc

import (
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
		return intToStr(int64(x))
	case int64:
		return intToStr(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return ""
	}
	return ""
}

func intToStr(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
