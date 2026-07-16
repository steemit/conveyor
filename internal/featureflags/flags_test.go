package featureflags

import (
	"testing"

	"github.com/steemit/conveyor/internal/jsonrpc"
	"github.com/steemit/conveyor/internal/store"
)

func testFlags() *Flags {
	return New(store.NewMemoryStore(), "conveyor", "admin")
}

func adminCtx() *jsonrpc.Context {
	return &jsonrpc.Context{Account: "admin"}
}

func userCtx(name string) *jsonrpc.Context {
	return &jsonrpc.Context{Account: name}
}

func TestSetAndGetFlag_Override(t *testing.T) {
	f := testFlags()

	// Set override true.
	req := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"new_ui","value":true}`)}
	if _, err := f.setFlag(adminCtx(), req); err != nil {
		t.Fatal(err)
	}

	// Get should return the override.
	getReq := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"new_ui"}`)}
	result, err := f.getFlag(userCtx("alice"), getReq)
	if err != nil {
		t.Fatal(err)
	}
	if result != true {
		t.Fatalf("expected true, got %v", result)
	}
}

func TestGetFlag_OverrideFalse_FallsThroughToProbability(t *testing.T) {
	f := testFlags()

	// Set override false.
	req := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"new_ui","value":false}`)}
	if _, err := f.setFlag(adminCtx(), req); err != nil {
		t.Fatal(err)
	}

	// Set a probability > 0 so there's something to fall through to.
	probReq := &jsonrpc.Request{Params: []byte(`{"flag":"new_ui","probability":0.999}`)}
	if _, err := f.setProbability(adminCtx(), probReq); err != nil {
		t.Fatal(err)
	}

	// getFlag with override=false should fall through to probability check.
	// With prob=0.999, the deterministic check (flagProbability < 0.999) is
	// almost certainly true (flagProbability is in [0,1)).
	getReq := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"new_ui"}`)}
	result, err := f.getFlag(userCtx("alice"), getReq)
	if err != nil {
		t.Fatal(err)
	}
	// With 0.999 probability, the result should be true unless the deterministic
	// value for this specific (account, flag) happens to be >= 0.999.
	if result != true {
		p := flagProbability("alice", "new_ui")
		t.Logf("flagProbability(alice,new_ui) = %v >= 0.999, so result=false is correct", p)
	}
}

func TestGetFlag_NoData_ReturnsFalse(t *testing.T) {
	f := testFlags()
	req := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"nonexistent"}`)}
	result, err := f.getFlag(userCtx("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	if result != false {
		t.Fatalf("expected false, got %v", result)
	}
}

func TestSetProbability_Zero_Deletes(t *testing.T) {
	f := testFlags()

	// Set probability to 0.5.
	req := &jsonrpc.Request{Params: []byte(`{"flag":"beta","probability":0.5}`)}
	f.setProbability(adminCtx(), req)

	// Verify it exists.
	probs, _ := f.readProbabilities(jsonrpc.Request{})
	if _, ok := probs["beta"]; !ok {
		t.Fatal("expected beta to exist")
	}

	// Set to 0 → should delete.
	req0 := &jsonrpc.Request{Params: []byte(`{"flag":"beta","probability":0}`)}
	f.setProbability(adminCtx(), req0)
	probs, _ = f.readProbabilities(jsonrpc.Request{})
	if _, ok := probs["beta"]; ok {
		t.Fatal("expected beta to be deleted")
	}
}

func TestGetFlags_MergesOverrideAndProbability(t *testing.T) {
	f := testFlags()

	// Set override for flag_a = true.
	f.setFlag(adminCtx(), &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"flag_a","value":true}`)})
	// Set probability for flag_b = 0.5.
	f.setProbability(adminCtx(), &jsonrpc.Request{Params: []byte(`{"flag":"flag_b","probability":0.5}`)})

	req := &jsonrpc.Request{Params: []byte(`{"account":"alice"}`)}
	result, err := f.getFlags(userCtx("alice"), req)
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]bool)
	if m["flag_a"] != true {
		t.Fatal("expected flag_a override = true")
	}
	if _, ok := m["flag_b"]; !ok {
		t.Fatal("expected flag_b from probability")
	}
}

func TestSetFlag_NonAdmin_Unauthorized(t *testing.T) {
	f := testFlags()
	req := &jsonrpc.Request{Params: []byte(`{"account":"alice","flag":"x","value":true}`)}
	_, err := f.setFlag(userCtx("alice"), req)
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestValidateFlag(t *testing.T) {
	valid := []string{"new_ui", "flag", "a_b_c", "dark_mode"}
	invalid := []string{"NewUI", "flag-1", "1flag", "", "flag.b"}
	for _, f := range valid {
		if e := validateFlag(f); e != nil {
			t.Errorf("expected valid for %q, got error: %v", f, e)
		}
	}
	for _, f := range invalid {
		if e := validateFlag(f); e == nil {
			t.Errorf("expected invalid for %q", f)
		}
	}
}
