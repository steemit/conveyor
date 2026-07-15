package store

import (
	"context"
	"errors"
	"testing"

	"github.com/steemit/conveyor/internal/config"
)

func TestMemoryStore_WriteRead(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	if err := s.Write(ctx, "k", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	b, err := s.Read(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("got %q, want 'hello'", b)
	}
}

func TestMemoryStore_ReadNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.Read(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMemoryStore_SafeReadNotFound(t *testing.T) {
	s := NewMemoryStore()
	b, err := s.SafeRead(context.Background(), "missing")
	if err != nil {
		t.Fatalf("SafeRead should not error on missing key: %v", err)
	}
	if b != nil {
		t.Fatalf("SafeRead should return nil for missing key, got %v", b)
	}
}

func TestMemoryStore_Overwrite(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_ = s.Write(ctx, "k", []byte("v1"))
	_ = s.Write(ctx, "k", []byte("v2"))
	b, _ := s.Read(ctx, "k")
	if string(b) != "v2" {
		t.Fatalf("expected overwrite to v2, got %q", b)
	}
}

func TestMemoryStore_ReadJSON_NotFound_TargetUntouched(t *testing.T) {
	s := NewMemoryStore()
	// Pre-initialise target to a default value.
	drafts := []any{"existing"}
	err := s.ReadJSON(context.Background(), "missing", &drafts)
	if err != nil {
		t.Fatalf("ReadJSON should not error on missing key: %v", err)
	}
	// Target should be untouched — the default value survives.
	if len(drafts) != 1 || drafts[0] != "existing" {
		t.Fatalf("target should be untouched, got %v", drafts)
	}
}

func TestMemoryStore_ReadJSON_Hit(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	_ = s.WriteJSON(ctx, "k", map[string]int{"a": 1, "b": 2})

	var m map[string]int
	if err := s.ReadJSON(ctx, "k", &m); err != nil {
		t.Fatal(err)
	}
	if m["a"] != 1 || m["b"] != 2 {
		t.Fatalf("unexpected value: %v", m)
	}
}

func TestMemoryStore_WriteReadJSON_RoundTrip(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	type payload struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	in := payload{Name: "test", N: 42}
	if err := s.WriteJSON(ctx, "k", in); err != nil {
		t.Fatal(err)
	}
	var out payload
	if err := s.ReadJSON(ctx, "k", &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

func TestNewStore_Memory(t *testing.T) {
	s, err := NewStore(context.Background(), config.StorageConfig{Type: "memory"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*MemoryStore); !ok {
		t.Fatalf("expected *MemoryStore, got %T", s)
	}
}

func TestNewStore_InvalidType(t *testing.T) {
	_, err := NewStore(context.Background(), config.StorageConfig{Type: "ftp"})
	if err == nil {
		t.Fatal("expected error for invalid storage type")
	}
}
