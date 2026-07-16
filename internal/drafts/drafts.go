// Package drafts implements the draft storage subsystem (list/save/remove),
// mirroring the original TS src/drafts.ts. Drafts are stored as JSON blobs in
// the BlobStore, keyed by account.
package drafts

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/store"
)

// Drafts holds the dependencies for draft operations.
type Drafts struct {
	store  store.BlobStore
	prefix string // config.Name, used as key prefix
}

// New creates a Drafts instance backed by the given store and key prefix.
func New(s store.BlobStore, prefix string) *Drafts {
	return &Drafts{store: s, prefix: prefix}
}

// Register registers the draft RPC methods on the given server. All three
// methods are authenticated (the account must match ctx.Account).
func (d *Drafts) Register(rpc *jsonrpc.Server) {
	rpc.RegisterAuthenticated("conveyor.list_drafts", d.list)
	rpc.RegisterAuthenticated("conveyor.save_draft", d.save)
	rpc.RegisterAuthenticated("conveyor.remove_draft", d.remove)
}

func (d *Drafts) draftsKey(account string) string {
	return fmt.Sprintf("%s_%s_drafts.json", d.prefix, account)
}

// readDrafts loads all drafts for an account. Returns an empty slice if the
// key does not exist (ReadJSON leaves target untouched when key is missing).
func (d *Drafts) readDrafts(ctx context.Context, account string) ([]any, error) {
	drafts := []any{}
	err := d.store.ReadJSON(ctx, d.draftsKey(account), &drafts)
	return drafts, err
}

func (d *Drafts) writeDrafts(ctx context.Context, account string, drafts []any) error {
	return d.store.WriteJSON(ctx, d.draftsKey(account), drafts)
}

func (d *Drafts) list(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account, "Unauthorized"); e != nil {
		return nil, e
	}
	ctx.Log.Info().Str("account", p.Account).Msg("list drafts")
	return d.readDrafts(req.Ctx, p.Account)
}

func (d *Drafts) save(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string          `json:"account"`
		Draft   map[string]any  `json:"draft"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account, "Unauthorized"); e != nil {
		return nil, e
	}
	if p.Draft == nil {
		p.Draft = map[string]any{}
	}
	// Generate uuid if missing.
	id, ok := p.Draft["uuid"].(string)
	if !ok || id == "" {
		id = uuid.NewString()
		p.Draft["uuid"] = id
	}
	ctx.Log.Info().Str("account", p.Account).Str("uuid", id).Msg("save draft")

	drafts, err := d.readDrafts(req.Ctx, p.Account)
	if err != nil {
		return nil, err
	}
	// Replace existing draft with same uuid, or append.
	found := false
	for i, item := range drafts {
		if m, ok := item.(map[string]any); ok {
			if existingID, _ := m["uuid"].(string); existingID == id {
				drafts[i] = p.Draft
				found = true
				break
			}
		}
	}
	if !found {
		drafts = append(drafts, p.Draft)
	}
	if err := d.writeDrafts(req.Ctx, p.Account, drafts); err != nil {
		return nil, err
	}
	return map[string]string{"uuid": id}, nil
}

func (d *Drafts) remove(ctx *jsonrpc.Context, req *jsonrpc.Request) (any, error) {
	var p struct {
		Account string `json:"account"`
		UUID    string `json:"uuid"`
	}
	if err := req.UnmarshalParams(&p); err != nil {
		return nil, err
	}
	if e := ctx.Assert(ctx.Account == p.Account, "Unauthorized"); e != nil {
		return nil, e
	}
	ctx.Log.Info().Str("account", p.Account).Str("uuid", p.UUID).Msg("remove draft")

	drafts, err := d.readDrafts(req.Ctx, p.Account)
	if err != nil {
		return nil, err
	}
	// Find and remove the draft with matching uuid.
	found := false
	for i, item := range drafts {
		if m, ok := item.(map[string]any); ok {
			if existingID, _ := m["uuid"].(string); existingID == p.UUID {
				drafts = append(drafts[:i], drafts[i+1:]...)
				found = true
				break
			}
		}
	}
	if !found {
		return nil, jsonrpc.NewErrorWithData(100, nil, "Draft not found", map[string]string{"uuid": p.UUID})
	}
	if err := d.writeDrafts(req.Ctx, p.Account, drafts); err != nil {
		return nil, err
	}
	return map[string]string{"uuid": p.UUID}, nil
}
