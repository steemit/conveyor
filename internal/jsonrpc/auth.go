package jsonrpc

import (
	"context"
	"encoding/json"
	"strings"
)

// Authenticator verifies a __signed JSON-RPC request and returns the decoded
// plaintext params plus the authenticated account name. Implementations live
// outside the jsonrpc package (e.g. internal/server) to avoid coupling this
// leaf package to steemgosdk/steemutil.
//
// On failure the returned *Error is propagated directly to the client —
// typically code 401 "Unauthorized: <cause>".
type Authenticator interface {
	Authenticate(ctx context.Context, method string, params json.RawMessage) (decodedParams json.RawMessage, account string, err *Error)
}

// hasSignedWrapper reports whether params contains a top-level __signed key.
// This mirrors the behavior of koa-jsonrpc's resolveParams, which would reject
// an authenticated method's request if the __signed param name is absent.
func hasSignedWrapper(params json.RawMessage) bool {
	if len(params) == 0 {
		return false
	}
	var probe struct {
		Signed json.RawMessage `json:"__signed"`
	}
	if err := json.Unmarshal(params, &probe); err != nil {
		return false
	}
	// Go unmarshals JSON null into a non-empty RawMessage ("null"), so check
	// for that explicitly.
	cleaned := strings.TrimSpace(string(probe.Signed))
	return len(cleaned) > 0 && cleaned != "null"
}
