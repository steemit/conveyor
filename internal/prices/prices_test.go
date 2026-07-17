package prices

import (
	"encoding/json"
	"strings"
	"testing"

	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// TestComputePrices_Conversion verifies that ComputePrices produces sane
// results from fixture data, and that the int64 → float64 conversion produces
// reasonable values. This tests the core math without needing a live steemd.
func TestComputePrices_Conversion(t *testing.T) {
	// Fixture: 1 ask + 1 bid (matching TS limit=1).
	// Ask: someone selling STEEM for SBD at 1:0.5 (base=5.000 SBD, quote=10.000 STEEM)
	// Bid: someone buying STEEM with SBD at the same ratio.
	ob := &protocolapi.OrderBook{
		Asks: []protocolapi.Order{
			{OrderPrice: protocolapi.OrderPrice{Base: "0.500 SBD", Quote: "1.000 STEEM"}},
		},
		Bids: []protocolapi.Order{
			{OrderPrice: protocolapi.OrderPrice{Base: "0.500 SBD", Quote: "1.000 STEEM"}},
		},
	}

	fh := &protocolapi.FeedHistory{
		PriceHistory: []protocolapi.CurrentMedianHistoryPrice{
			{Base: "0.500 SBD", Quote: "1.000 STEEM"},
		},
	}

	dgp := &protocolapi.DynamicGlobalProperties{
		TotalVestingFundSteem: "100.000 STEEM",
		TotalVestingShares:    "4200000.000000 VESTS",
	}

	result, err := protocolapi.ComputePrices(ob, fh, dgp)
	if err != nil {
		t.Fatalf("ComputePrices failed: %v", err)
	}

	// steem_sbd: mean of Convert(1 STEEM) over 2 orders, each yielding 0.500 SBD.
	// → 500 (atomic, 3 decimals) → 0.5 float.
	sbd := float64(result.SteemSbd.Amount) / 1e3
	if sbd != 0.5 {
		t.Errorf("steem_sbd: expected 0.5, got %f (raw=%d)", sbd, result.SteemSbd.Amount)
	}

	// steem_usd: Price 0.500 SBD / 1.000 STEEM, convert 1 STEEM → 0.500 SBD.
	oneSteem := protocolapi.Asset{Amount: 1000, Symbol: "STEEM"}
	usdAsset, err := result.SteemUsd.Convert(oneSteem)
	if err != nil {
		t.Fatalf("SteemUsd.Convert failed: %v", err)
	}
	usd := float64(usdAsset.Amount) / 1e3
	if usd != 0.5 {
		t.Errorf("steem_usd: expected 0.5, got %f (raw=%d)", usd, usdAsset.Amount)
	}

	// steem_vest: Price 100.000 STEEM / 4200000.000000 VESTS.
	// Convert 1 STEEM → 42000 VESTS → 42000000000 (atomic, 6 decimals) → 42000.0.
	vestAsset, err := result.SteemVest.Convert(oneSteem)
	if err != nil {
		t.Fatalf("SteemVest.Convert failed: %v", err)
	}
	vest := float64(vestAsset.Amount) / 1e6
	// 100 STEEM = 4200000 VESTS → 1 STEEM = 42000 VESTS
	if vest != 42000.0 {
		t.Errorf("steem_vest: expected 42000.0, got %f (raw=%d)", vest, vestAsset.Amount)
	}

	t.Logf("steem_sbd=%f, steem_usd=%f, steem_vest=%f", sbd, usd, vest)
}

func TestPricesResponse_JSON(t *testing.T) {
	// Verify the response struct has the correct JSON tags.
	r := pricesResponse{SteemSbd: 0.5, SteemUsd: 0.827, SteemVest: 2019.1}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, key := range []string{`"steem_sbd"`, `"steem_usd"`, `"steem_vest"`} {
		if !strings.Contains(s, key) {
			t.Errorf("missing %s in JSON: %s", key, s)
		}
	}
}
