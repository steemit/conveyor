package drafts

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/store"
)

func testDrafts(t *testing.T) *Drafts {
	return New(store.NewMemoryStore(), "conveyor")
}

func testCtx(account string) *jsonrpc.Context {
	return &jsonrpc.Context{Account: account}
}

func TestList_Empty(t *testing.T) {
	d := testDrafts(t)
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice"}`)}
	result, err := d.list(testCtx("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := result.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", result)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty, got %d", len(arr))
	}
}

func TestSaveAndList(t *testing.T) {
	d := testDrafts(t)
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"title":"hello","body":"world"}}`)}
	resp, err := d.save(testCtx("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	m := resp.(map[string]string)
	if m["uuid"] == "" {
		t.Fatal("expected non-empty uuid")
	}

	// List should return 1 draft.
	listReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice"}`)}
	result, err := d.list(testCtx("alice"), listReq)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.([]any)) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(result.([]any)))
	}
}

func TestSave_ExistingUUID_Updates(t *testing.T) {
	d := testDrafts(t)
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"uuid":"fixed-uuid","title":"v1"}}`)}
	_, _ = d.save(testCtx("alice"), req)
	// Save again with same uuid, different title.
	req2 := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"uuid":"fixed-uuid","title":"v2"}}`)}
	_, err := d.save(testCtx("alice"), req2)
	if err != nil {
		t.Fatal(err)
	}
	listReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice"}`)}
	result, _ := d.list(testCtx("alice"), listReq)
	arr := result.([]any)
	if len(arr) != 1 {
		t.Fatalf("expected 1 draft (update), got %d", len(arr))
	}
	m := arr[0].(map[string]any)
	if m["title"] != "v2" {
		t.Fatalf("expected title v2, got %v", m["title"])
	}
}

func TestRemove(t *testing.T) {
	d := testDrafts(t)
	saveReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"title":"x"}}`)}
	resp, _ := d.save(testCtx("alice"), saveReq)
	id := resp.(map[string]string)["uuid"]

	rmReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","uuid":"` + id + `"}`)}
	_, err := d.remove(testCtx("alice"), rmReq)
	if err != nil {
		t.Fatal(err)
	}
	// List should be empty.
	listReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice"}`)}
	result, _ := d.list(testCtx("alice"), listReq)
	if len(result.([]any)) != 0 {
		t.Fatalf("expected empty after remove, got %d", len(result.([]any)))
	}
}

func TestRemove_NotFound(t *testing.T) {
	d := testDrafts(t)
	rmReq := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","uuid":"nonexistent"}`)}
	_, err := d.remove(testCtx("alice"), rmReq)
	if err == nil {
		t.Fatal("expected error for non-existent draft")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 100 {
		t.Fatalf("expected code 100, got %v", err)
	}
}

func TestList_UnauthorizedOtherAccount(t *testing.T) {
	d := testDrafts(t)
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice"}`)}
	_, err := d.list(testCtx("bob"), req) // ctx.Account=bob, but params account=alice
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

// TestSave_CountCap verifies that appending past maxDraftsPerAccount fails,
// while replacing an existing uuid still succeeds (audit 2026-08-18 T-010).
func TestSave_CountCap(t *testing.T) {
	d := testDrafts(t)
	for i := 0; i < maxDraftsPerAccount; i++ {
		req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"uuid":"d` + strconv.Itoa(i) + `","t":"x"}}`)}
		if _, err := d.save(testCtx("alice"), req); err != nil {
			t.Fatalf("save %d unexpected error: %v", i, err)
		}
	}
	// One more append must fail.
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"uuid":"overflow","t":"x"}}`)}
	_, err := d.save(testCtx("alice"), req)
	if err == nil {
		t.Fatal("expected error when exceeding draft count cap")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
	// Replacing an existing uuid must still work.
	upd := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(`{"account":"alice","draft":{"uuid":"d0","t":"y"}}`)}
	if _, err := d.save(testCtx("alice"), upd); err != nil {
		t.Fatalf("replace at cap should succeed, got: %v", err)
	}
}

// TestSave_SizeCap verifies that a draft set whose serialized blob exceeds
// maxDraftsBytes is rejected (audit 2026-08-18 T-010).
func TestSave_SizeCap(t *testing.T) {
	d := testDrafts(t)
	big := strings.Repeat("x", maxDraftsBytes) // alone exceeds the blob cap
	params := fmt.Sprintf(`{"account":"alice","draft":{"uuid":"big","body":"%s"}}`, big)
	req := &jsonrpc.Request{Ctx: context.Background(), Params: []byte(params)}
	_, err := d.save(testCtx("alice"), req)
	if err == nil {
		t.Fatal("expected error when draft blob exceeds byte cap")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}
