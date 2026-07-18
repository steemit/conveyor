// Package prices implements the conveyor.get_prices RPC method, mirroring the
// original TS src/price.ts. It fetches order book, feed history, and dynamic
// global properties from steemd in parallel, then uses steemutil's ComputePrices
// (int64 + math/big arithmetic) to derive the three price values.
package prices

import (
	"sync"

	"github.com/steemit/steemgosdk/api"
	protocolapi "github.com/steemit/steemutil/protocol/api"

	"github.com/steemit/conveyor/internal/jsonrpc"
)

// Prices holds the steemd API client used to fetch market data.
type Prices struct {
	api *api.API
}

// New creates a Prices instance backed by the given Steem RPC node.
func New(rpcNode string) *Prices {
	return &Prices{api: api.NewAPI(rpcNode)}
}

// Register registers the conveyor.get_prices method (public, no auth).
func (p *Prices) Register(rpc *jsonrpc.Server) {
	rpc.Register("conveyor.get_prices", p.GetPrices)
}

// pricesResponse is the JSON-RPC result shape, matching the schema's
// "prices" definition: three float64 fields.
type pricesResponse struct {
	SteemSbd  float64 `json:"steem_sbd"`
	SteemUsd  float64 `json:"steem_usd"`
	SteemVest float64 `json:"steem_vest"`
}

func (p *Prices) GetPrices(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	// Fetch the three data sources in parallel (matching TS Promise.all).
	var (
		ob  *protocolapi.OrderBook
		fh  *protocolapi.FeedHistory
		dgp *protocolapi.DynamicGlobalProperties
		obErr, fhErr, dgpErr error
		wg  sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		ob, obErr = p.api.GetOrderBook(1)
	}()
	go func() {
		defer wg.Done()
		fh, fhErr = p.api.GetFeedHistory()
	}()
	go func() {
		defer wg.Done()
		dgp, dgpErr = p.api.GetDynamicGlobalProperties()
	}()
	wg.Wait()

	if obErr != nil {
		return nil, jsonrpc.ErrInternalError(obErr)
	}
	if fhErr != nil {
		return nil, jsonrpc.ErrInternalError(fhErr)
	}
	if dgpErr != nil {
		return nil, jsonrpc.ErrInternalError(dgpErr)
	}

	// Compute prices using steemutil's integer-arithmetic implementation.
	result, err := protocolapi.ComputePrices(ob, fh, dgp)
	if err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}

	// Convert int64 atomic units to float64 display values.
	//
	// SteemSbd: already the averaged conversion result (Asset), divide by
	//   10^3 (SBD precision).
	// SteemUsd: a raw Price (not pre-converted). Must Convert(1 STEEM) to get
	//   the SBD amount, then divide by 10^3.
	// SteemVest: a raw Price. Must Convert(1 STEEM) to get the VESTS amount,
	//   then divide by 10^6 (VESTS precision).
	oneSteem := protocolapi.Asset{Amount: 1000, Symbol: "STEEM"} // 1.000 STEEM

	usdAsset, err := result.SteemUsd.Convert(oneSteem)
	if err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}
	vestAsset, err := result.SteemVest.Convert(oneSteem)
	if err != nil {
		return nil, jsonrpc.ErrInternalError(err)
	}

	return pricesResponse{
		SteemSbd:  float64(result.SteemSbd.Amount) / 1e3,
		SteemUsd:  float64(usdAsset.Amount) / 1e3,
		SteemVest: float64(vestAsset.Amount) / 1e6,
	}, nil
}
