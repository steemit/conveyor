package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// maxRequestBodyBytes caps the size of a single JSON-RPC request body.
// JSON-RPC requests are small; this bounds memory use and prevents trivial
// memory-exhaustion DoS. Batches are subject to the same cap. A body exceeding
// the cap fails io.ReadAll with an *http.MaxBytesError and is surfaced as a
// 400 ParseError by the read-error path below.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

// maxBatchItems caps the number of sub-requests accepted in a single JSON-RPC
// batch. Without a cap, one sub-1-MiB body can carry thousands of sub-requests
// which are all dispatched concurrently, amplifying a single unauthenticated
// request into thousands of upstream calls (audit 2026-08-18 T-001).
const maxBatchItems = 50

// Handler returns a gin.HandlerFunc that serves JSON-RPC 2.0 over POST /.
// Behavior mirrors @steemit/koa-jsonrpc's middleware:
//   - body parse failure -> 400 + ParseError
//   - empty array -> 400 + InvalidRequest
//   - batch -> 200, concurrent handling, notifications filtered out
//   - single notification -> 200 with empty body
//
// Note: method routing (POST vs GET) is handled by the Gin router — this
// handler is only registered on POST /. The original koa-jsonrpc had an
// internal 405 check; we rely on Gin's routing instead.
func (s *Server) Handler(log zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Cap the request body to bound memory use. An oversized body makes
		// io.ReadAll fail (the writer gets a 413-like signal) and flows into
		// the same 400 ParseError path as other read failures.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodyBytes)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			respondError(c, http.StatusBadRequest, &Response{
				JSONRPC: "2.0", ID: ID{kind: idNull},
				Error: ErrParseError(err),
			})
			return
		}

		// Determine array vs object by trimming whitespace.
		trimmed := trimLeftWhitespace(body)
		if len(trimmed) == 0 {
			// Empty body -> ParseError (no valid JSON).
			respondError(c, http.StatusBadRequest, &Response{
				JSONRPC: "2.0", ID: ID{kind: idNull},
				Error: ErrParseError(nil).withMessage("Parse error: empty body"),
			})
			return
		}

		ctx := c.Request.Context()
		clientIP := c.ClientIP()

		if trimmed[0] == '[' {
			var items []json.RawMessage
			if err := json.Unmarshal(body, &items); err != nil {
				respondError(c, http.StatusBadRequest, &Response{
					JSONRPC: "2.0", ID: ID{kind: idNull},
					Error: ErrParseError(err),
				})
				return
			}
			if len(items) == 0 {
				respondError(c, http.StatusBadRequest, &Response{
					JSONRPC: "2.0", ID: ID{kind: idNull},
					Error: ErrInvalidRequest(nil),
				})
				return
			}
			if len(items) > maxBatchItems {
				respondError(c, http.StatusBadRequest, &Response{
					JSONRPC: "2.0", ID: ID{kind: idNull},
					Error: ErrInvalidRequest(nil).withMessage(
						fmt.Sprintf("Invalid Request: batch exceeds %d items", maxBatchItems)),
				})
				return
			}
			responses := s.handleBatch(ctx, items, log, clientIP)
			writeBatch(c, responses)
			return
		}

		// Single request: validate JSON before dispatching. A top-level
		// unmarshal failure is a parse error (HTTP 400), not a normal
		// InvalidRequest response (which would be HTTP 200).
		var single json.RawMessage
		if err := json.Unmarshal(body, &single); err != nil {
			respondError(c, http.StatusBadRequest, &Response{
				JSONRPC: "2.0", ID: ID{kind: idNull},
				Error: ErrParseError(err),
			})
			return
		}
		resp := s.dispatch(ctx, single, log, clientIP)
		if resp == nil {
			// Notification: empty body, 200.
			c.Status(http.StatusOK)
			c.Header("Content-Type", "application/json")
			_, _ = c.Writer.Write(nil)
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

// withMessage overrides the message (used for static error strings like
// "Method Not Allowed" where there's no cause to append).
func (e *Error) withMessage(msg string) *Error {
	e.Message = msg
	return e
}

func writeBatch(c *gin.Context, responses []*Response) {
	c.Header("Content-Type", "application/json")
	if len(responses) == 0 {
		// All notifications: empty body.
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(nil)
		return
	}
	c.Status(http.StatusOK)
	b, err := json.Marshal(responses)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	_, _ = c.Writer.Write(b)
}

func respondError(c *gin.Context, status int, resp *Response) {
	c.Header("Content-Type", "application/json")
	c.Status(status)
	b, _ := json.Marshal(resp)
	_, _ = c.Writer.Write(b)
}

func trimLeftWhitespace(b []byte) []byte {
	i := 0
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return b[i:]
}
