package userdata

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/models"
)

func testUserData(t *testing.T) *UserData {
	db, err := models.NewTestDB()
	if err != nil {
		t.Fatal(err)
	}
	return New(db, "admin")
}

func adminCtx() *jsonrpc.Context { return &jsonrpc.Context{Account: "admin"} }
func userCtx(name string) *jsonrpc.Context { return &jsonrpc.Context{Account: name} }

func TestSetAndGetUserData(t *testing.T) {
	u := testUserData(t)

	// Set.
	req := &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":"alice@example.com","phone":"+1234567890"}}`)}
	if _, err := u.setUserData(userCtx("alice"), req); err != nil {
		t.Fatal(err)
	}

	// Get.
	getReq := &jsonrpc.Request{Params: []byte(`{"account":"alice"}`)}
	result, err := u.getUserData(userCtx("alice"), getReq)
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]any)
	if m["email"] != "alice@example.com" {
		t.Fatalf("expected email, got %v", m["email"])
	}
	if m["phone"] != "+1234567890" {
		t.Fatalf("expected phone, got %v", m["phone"])
	}
}

func TestGetUserData_NotFound(t *testing.T) {
	u := testUserData(t)
	req := &jsonrpc.Request{Params: []byte(`{"account":"ghost"}`)}
	_, err := u.getUserData(userCtx("ghost"), req)
	if err == nil {
		t.Fatal("expected 404")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 404 {
		t.Fatalf("expected code 404, got %v", err)
	}
}

func TestSetUserData_Update(t *testing.T) {
	u := testUserData(t)

	// Create.
	req1 := &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":"a@b.com"}}`)}
	u.setUserData(userCtx("alice"), req1)

	// Update phone.
	req2 := &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"phone":"+999"}}`)}
	if _, err := u.setUserData(userCtx("alice"), req2); err != nil {
		t.Fatal(err)
	}

	getReq := &jsonrpc.Request{Params: []byte(`{"account":"alice"}`)}
	result, _ := u.getUserData(userCtx("alice"), getReq)
	m := result.(map[string]any)
	if m["phone"] != "+999" {
		t.Fatalf("expected updated phone, got %v", m["phone"])
	}
	// Email should still be there (partial update).
	if m["email"] != "a@b.com" {
		t.Fatalf("expected email to persist, got %v", m["email"])
	}
}

func TestSetUserData_UniqueEmailConflict(t *testing.T) {
	u := testUserData(t)

	// User 1 with email.
	u.setUserData(adminCtx(), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":"shared@example.com"}}`)})
	// User 2 with same email → should fail.
	_, err := u.setUserData(adminCtx(), &jsonrpc.Request{Params: []byte(`{"account":"bob","userData":{"email":"shared@example.com"}}`)})
	if err == nil {
		t.Fatal("expected unique constraint error")
	}
	// Must be a 400.
	rpcErr, ok := err.(*jsonrpc.Error)
	if !ok || rpcErr.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
	// The raw driver error (table/column/dialect detail) must NOT leak to the
	// client — only a generic "must be unique" message is acceptable.
	serialized, _ := json.Marshal(rpcErr)
	if strings.Contains(string(serialized), "UNIQUE") || strings.Contains(string(serialized), "duplicate key") {
		t.Fatalf("raw DB error leaked to client: %s", serialized)
	}
}

func TestSetUserData_InvalidEmail(t *testing.T) {
	u := testUserData(t)
	_, err := u.setUserData(userCtx("alice"), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":"not-an-email"}}`)})
	if err == nil {
		t.Fatal("expected invalid email error")
	}
}

func TestSetUserData_InvalidPhone(t *testing.T) {
	u := testUserData(t)
	_, err := u.setUserData(userCtx("alice"), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"phone":"1234567890"}}`)})
	if err == nil {
		t.Fatal("expected invalid phone error (must start with +)")
	}
}

// TestSetUserData_NonStringEmail verifies that a non-string email value does
// not panic and returns a clean 400 (previously v.(string) would panic).
func TestSetUserData_NonStringEmail(t *testing.T) {
	u := testUserData(t)
	_, err := u.setUserData(userCtx("alice"), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":123}}`)})
	if err == nil {
		t.Fatal("expected error for non-string email")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}

// TestSetUserData_NonStringPhone verifies that a non-string phone value does
// not panic and returns a clean 400.
func TestSetUserData_NonStringPhone(t *testing.T) {
	u := testUserData(t)
	_, err := u.setUserData(userCtx("alice"), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"phone":456}}`)})
	if err == nil {
		t.Fatal("expected error for non-string phone")
	}
	e, ok := err.(*jsonrpc.Error)
	if !ok || e.Code != 400 {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestIsEmailRegistered(t *testing.T) {
	u := testUserData(t)
	u.setUserData(adminCtx(), &jsonrpc.Request{Params: []byte(`{"account":"alice","userData":{"email":"alice@example.com"}}`)})

	req := &jsonrpc.Request{Params: []byte(`{"email":"alice@example.com"}`)}
	result, err := u.isEmailRegistered(adminCtx(), req)
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Fatal("expected email to be registered")
	}

	// Unregistered email.
	req2 := &jsonrpc.Request{Params: []byte(`{"email":"nobody@example.com"}`)}
	result2, _ := u.isEmailRegistered(adminCtx(), req2)
	if result2 != false {
		t.Fatal("expected email to NOT be registered")
	}
}

func TestIsEmailRegistered_NonAdmin_Unauthorized(t *testing.T) {
	u := testUserData(t)
	_, err := u.isEmailRegistered(userCtx("alice"), &jsonrpc.Request{Params: []byte(`{"email":"x@y.com"}`)})
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}
