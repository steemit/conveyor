// Package featureflags implements the feature-flag subsystem, mirroring the
// original TS src/feature-flags.ts. Flags are stored as JSON blobs (per-user
// overrides + global probabilities). Probabilistic flag assignment uses a
// bit-for-bit port of random-js v1's MT19937.
package featureflags

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"regexp"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/store"
)

var flagPattern = regexp.MustCompile(`^[a-z_]+$`)

// Flags holds the dependencies for feature-flag operations.
type Flags struct {
	store      store.BlobStore
	prefix     string
	adminRole  string
}

// New creates a Flags instance.
func New(s store.BlobStore, prefix, adminRole string) *Flags {
	return &Flags{store: s, prefix: prefix, adminRole: adminRole}
}

// Register registers all feature-flag RPC methods.
//
// KNOWN LIMITATION (audit 2026-08-18 T-011, accepted 2026-08-27): grouping is
// fully deterministic and unsalted — flagProbability hashes only the PUBLIC
// inputs (account + flag name), and the threshold is the sole secret. Anyone
// controlling N accounts can recover the threshold offline (call
// get_feature_flags for their own accounts, bracket the N on/off boundary)
// and then predict EVERY account's grouping without touching the server;
// values are identical across deployments (no per-deployment salt), so
// precomputed tables are portable.
//
// Currently NO consumer calls these methods in production (condenser-legacy's
// get_feature_flags path is disabled via an early return; faucet and the new
// condenser never call them), so the exposure is dormant.
//
// BEFORE putting these methods into real use: derive the probability from
// SHA256(serverSecretSalt + account + flag) with the salt injected from the
// environment/secret store (never stored in the blob), and accept that
// enabling a salt re-shuffles every account's grouping once.
func (f *Flags) Register(rpc *jsonrpc.Server) {
	rpc.RegisterAuthenticated("conveyor.get_feature_flag", f.getFlag)
	rpc.RegisterAuthenticated("conveyor.set_feature_flag", f.setFlag)
	rpc.RegisterAuthenticated("conveyor.get_feature_flags", f.getFlags)
	rpc.RegisterAuthenticated("conveyor.set_feature_flag_probability", f.setProbability)
	rpc.RegisterAuthenticated("conveyor.get_feature_flag_probabilities", f.getProbabilities)
}

func (f *Flags) probabilityKey() string {
	return fmt.Sprintf("%s_feature-flags.json", f.prefix)
}

func (f *Flags) flagKey(account string) string {
	return fmt.Sprintf("%s_%s_feature-flags.json", f.prefix, account)
}

func (f *Flags) readProbabilities(ctx jsonrpc.Request) (map[string]float64, error) {
	m := map[string]float64{}
	err := f.store.ReadJSON(ctx.Ctx, f.probabilityKey(), &m)
	return m, err
}

func (f *Flags) writeProbabilities(ctx jsonrpc.Request, m map[string]float64) error {
	return f.store.WriteJSON(ctx.Ctx, f.probabilityKey(), m)
}

func (f *Flags) readFlags(ctx jsonrpc.Request, account string) (map[string]bool, error) {
	m := map[string]bool{}
	err := f.store.ReadJSON(ctx.Ctx, f.flagKey(account), &m)
	return m, err
}

func (f *Flags) writeFlags(ctx jsonrpc.Request, account string, m map[string]bool) error {
	return f.store.WriteJSON(ctx.Ctx, f.flagKey(account), m)
}

func validateFlag(flag string) *jsonrpc.Error {
	if !flagPattern.MatchString(flag) {
		return jsonrpc.NewErrorWithData(400, nil,
			"Flags must be lowercase and contain only a-z and underscore",
			map[string]string{"flag": flag})
	}
	return nil
}

// flagProbability computes the deterministic probability for an (account, flag)
// pair using the same MT19937 algorithm as random-js v1's realZeroToOneExclusive.
func flagProbability(account, flag string) float64 {
	hash := sha256.Sum256([]byte(account + flag))

	// Extract 8 int32 seeds from the hash (little-endian, matching Node's
	// Buffer.readInt32LE).
	seeds := make([]int32, 8)
	for i := 0; i < 8; i++ {
		seeds[i] = int32(binary.LittleEndian.Uint32(hash[i*4 : (i+1)*4]))
	}

	mt := newMT19937()
	mt.seedWithArray(seeds)
	return mt.realZeroToOneExclusive()
}

// --- RPC handlers ---

func (f *Flags) getFlag(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
		Flag    string `json:"flag"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account || ctx.Account == f.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}
	if e := validateFlag(p.Flag); e != nil {
		return nil, e
	}

	flags, err := f.readFlags(*req, p.Account)
	if err != nil {
		return nil, err
	}
	// JS truthy behavior: if flags[flag] is truthy (true), return it.
	// If it's false or absent, fall through to probability check.
	if val, ok := flags[p.Flag]; ok && val {
		return val, nil
	}

	probs, err := f.readProbabilities(*req)
	if err != nil {
		return nil, err
	}
	if prob, ok := probs[p.Flag]; ok {
		return flagProbability(p.Account, p.Flag) < prob, nil
	}
	return false, nil
}

func (f *Flags) setFlag(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
		Flag    string `json:"flag"`
		Value   *bool  `json:"value"` // null → delete override
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == f.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}
	if e := validateFlag(p.Flag); e != nil {
		return nil, e
	}
	if p.Value == nil {
		// Can't easily distinguish "not present" from "null" in Go; check raw JSON.
		// The TS asserts value is boolean or null. If value field is absent,
		// p.Value stays nil. TS: if value is null → delete.
		// We treat nil as "delete" (matching TS null behavior).
	}

	flags, err := f.readFlags(*req, p.Account)
	if err != nil {
		return nil, err
	}
	if p.Value == nil {
		ctx.Log.Info().Str("flag", p.Flag).Str("account", p.Account).Msg("deleting flag")
		delete(flags, p.Flag)
	} else {
		ctx.Log.Info().Str("flag", p.Flag).Str("account", p.Account).Bool("value", *p.Value).Msg("setting flag")
		flags[p.Flag] = *p.Value
	}
	if err := f.writeFlags(*req, p.Account, flags); err != nil {
		return nil, err
	}
	return map[string]any{}, nil
}

func (f *Flags) getFlags(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account || ctx.Account == f.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}

	rv, err := f.readFlags(*req, p.Account)
	if err != nil {
		return nil, err
	}
	probs, err := f.readProbabilities(*req)
	if err != nil {
		return nil, err
	}
	// Merge: for each probability flag, if no override exists, compute it.
	for flag, prob := range probs {
		if _, exists := rv[flag]; !exists {
			rv[flag] = flagProbability(p.Account, flag) < prob
		}
	}
	return rv, nil
}

func (f *Flags) setProbability(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Flag        string  `json:"flag"`
		Probability float64 `json:"probability"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == f.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}
	if e := validateFlag(p.Flag); e != nil {
		return nil, e
	}
	// Validate probability range.
	if p.Probability < 0 || p.Probability > 1 {
		return nil, jsonrpc.NewError(400, nil, "Probability must be a fraction between 0 and 1")
	}

	probs, err := f.readProbabilities(*req)
	if err != nil {
		return nil, err
	}
	if p.Probability == 0 {
		delete(probs, p.Flag)
	} else {
		probs[p.Flag] = p.Probability
	}
	if err := f.writeProbabilities(*req, probs); err != nil {
		return nil, err
	}
	return map[string]any{}, nil
}

func (f *Flags) getProbabilities(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	if e := ctx.Assert(ctx.Account == f.adminRole, "Unauthorized"); e != nil {
		return nil, e
	}
	return f.readProbabilities(*req)
}
