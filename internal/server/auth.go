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
