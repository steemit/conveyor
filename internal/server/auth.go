package server

import (
	"context"
	"encoding/json"

	"github.com/steemit/steemgosdk/api"
	"github.com/steemit/steemutil/rpc"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

// conveyorAuthenticator implements jsonrpc.Authenticator by delegating to
// steemgosdk's API.VerifySignedRequest, which internally calls steemutil's
// rpc.Validate (envelope/freshness/decode) + rpc.VerifySignedRpc (secp256k1
// recovery against the account's posting key fetched via get_accounts).
//
// KNOWN LIMITATION — replay within the freshness window:
//
// rpc.Validate checks the nonce FORMAT (8-byte hex) and the timestamp
// freshness (60s), but does NOT track nonce uniqueness — a captured signed
// request can be replayed verbatim within 60 seconds. This matches the
// original TS service (@steemit/koa-jsonrpc + @steemit/rpc-auth) and is
// accepted because every current RPC method is idempotent (or near- enough
// that a replay converges to the same state).
//
// If a NON-idempotent method is ever added (counters, transfers, one-time
// token consumption, ...), replay protection becomes mandatory:
//
//   - Track used nonces keyed by (account, nonce, timestamp) with a TTL of
//     ~60s (the freshness window).
//   - In a multi-instance deployment, an in-process cache is NOT sufficient
//     (a replay hitting a different instance would bypass it) — use a SHARED
//     store such as Redis.
//   - Reject duplicates with a 401 "replayed request" before invoking the
//     handler, right after VerifySignedRequest succeeds below.
type conveyorAuthenticator struct {
	api *api.API
}

// newAuthenticator builds an Authenticator backed by the given Steem RPC node.
func newAuthenticator(rpcNode string) jsonrpc.Authenticator {
	return &conveyorAuthenticator{api: api.NewAPI(rpcNode)}
}

// Authenticate extracts the __signed wrapper from params, constructs a
// rpc.SignedRequest (bypassing the int-typed ID field — Validate doesn't read
// ID), and delegates to API.VerifySignedRequest. On success it re-serializes
// the decoded params (interface{}) to json.RawMessage so the handler can
// UnmarshalParams them normally.
func (a *conveyorAuthenticator) Authenticate(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, string, *jsonrpc.Error) {
	// Extract the __signed payload. We use an intermediate struct because
	// rpc.SignedRequest.ID is int — unmarshaling a string/null id would fail.
	var wrapper struct {
		Signed rpc.SignedParams `json:"__signed"`
	}
	if err := json.Unmarshal(params, &wrapper); err != nil {
		return nil, "", jsonrpc.NewError(401, err, "Unauthorized")
	}

	// Construct a SignedRequest for VerifySignedRequest. ID is irrelevant —
	// rpc.Validate never reads it.
	signedReq := &rpc.SignedRequest{
		JsonRpc: "2.0",
		Method:  method,
		ID:      0,
	}
	signedReq.Params.Signed = wrapper.Signed

	decoded, account, err := a.api.VerifySignedRequest(signedReq)
	if err != nil {
		return nil, account, jsonrpc.NewError(401, err, "Unauthorized")
	}

	// Replay protection would go here (see the KNOWN LIMITATION note above):
	// reject a (account, nonce, timestamp) tuple that has already been seen.
	// Deferred until a non-idempotent method requires it; would need Redis
	// (or another shared store) to work across instances.

	// Re-serialize the decoded params (interface{}) to json.RawMessage so the
	// handler's UnmarshalParams works normally.
	decodedRaw, marshalErr := json.Marshal(decoded)
	if marshalErr != nil {
		return nil, account, jsonrpc.ErrInternalError(marshalErr)
	}

	return decodedRaw, account, nil
}

// Compile-time assertion that conveyorAuthenticator satisfies the interface.
var _ jsonrpc.Authenticator = (*conveyorAuthenticator)(nil)
