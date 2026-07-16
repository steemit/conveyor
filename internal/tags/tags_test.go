package tags

import (
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/models"
)

func testTags(t *testing.T) *Tags {
	db, err := models.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	return New(db, "admin")
}

func adminCtx() *jsonrpc.Context  { return &jsonrpc.Context{Account: "admin"} }
func userCtx() *jsonrpc.Context   { return &jsonrpc.Context{Account: "user"} }

func TestDefineAndListTags(t *testing.T) {
	tg := testTags(t)

	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"abuse","description":"bad actor"}`)})
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"verified","description":"verified"}`)})

	result, err := tg.listTags(adminCtx(), &jsonrpc.Request{})
	if err != nil {
		t.Fatal(err)
	}
	tags := result.([]map[string]string)
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0]["name"] != "abuse" {
		t.Fatalf("expected abuse first (sorted), got %s", tags[0]["name"])
	}
}

func TestDefineTag_InvalidName(t *testing.T) {
	tg := testTags(t)
	_, err := tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"Bad-Name","description":"x"}`)})
	if err == nil {
		t.Fatal("expected invalid name error")
	}
}

func TestAssignAndUnassign(t *testing.T) {
	tg := testTags(t)
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"verified","description":"v"}`)})

	// Assign.
	_, err := tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"verified"}`)})
	if err != nil {
		t.Fatal(err)
	}

	// Verify via getTagsForUser.
	result, _ := tg.getTagsForUser(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","audit":false}`)})
	tags := result.([]string)
	if len(tags) != 1 || tags[0] != "verified" {
		t.Fatalf("expected [verified], got %v", tags)
	}

	// Unassign.
	tg.unassignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"verified"}`)})

	// Active should be empty.
	result, _ = tg.getTagsForUser(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","audit":false}`)})
	if len(result.([]string)) != 0 {
		t.Fatalf("expected empty after unassign, got %v", result)
	}

	// Audit should still show it.
	result, _ = tg.getTagsForUser(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","audit":true}`)})
	arr := result.([]map[string]any)
	if len(arr) != 1 {
		t.Fatalf("audit should show 1 (soft-deleted), got %d", len(arr))
	}
}

func TestAssign_NonExistentTag(t *testing.T) {
	tg := testTags(t)
	_, err := tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"ghost"}`)})
	if err == nil {
		t.Fatal("expected error for non-existent tag")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 420 {
		t.Fatalf("expected code 420, got %v", err)
	}
}

func TestAssign_Idempotent(t *testing.T) {
	tg := testTags(t)
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"v","description":"d"}`)})

	tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"v"}`)})
	// Second assign should not error (idempotent).
	_, err := tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"v"}`)})
	if err != nil {
		t.Fatalf("idempotent assign should not error: %v", err)
	}
}

func TestGetUsersByTags_Intersection(t *testing.T) {
	tg := testTags(t)
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"t1","description":"d"}`)})
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"t2","description":"d"}`)})

	// alice has both tags.
	tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"t1"}`)})
	tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"t2"}`)})
	// bob has only t1.
	tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"bob","tag":"t1"}`)})

	// Query for users with BOTH t1 and t2 → only alice.
	result, err := tg.getUsersByTags(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tags":["t1","t2"]}`)})
	if err != nil {
		t.Fatal(err)
	}
	users := result.([]string)
	if len(users) != 1 || users[0] != "alice" {
		t.Fatalf("expected [alice], got %v", users)
	}

	// Query for single tag → both alice and bob.
	result, _ = tg.getUsersByTags(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tags":"t1"}`)})
	users = result.([]string)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestGetUsersByTags_AfterUnassign(t *testing.T) {
	tg := testTags(t)
	tg.defineTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"v","description":"d"}`)})
	tg.assignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"v"}`)})
	tg.unassignTag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"uid":"alice","tag":"v"}`)})

	// Unassigned users should not appear.
	result, _ := tg.getUsersByTags(adminCtx(), &jsonrpc.Request{Params: []byte(`{"tags":"v"}`)})
	if len(result.([]string)) != 0 {
		t.Fatalf("expected empty after unassign, got %v", result)
	}
}

func TestNonAdmin_Unauthorized(t *testing.T) {
	tg := testTags(t)
	_, err := tg.defineTag(userCtx(), &jsonrpc.Request{Params: []byte(`{"tag":"x","description":"d"}`)})
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}
