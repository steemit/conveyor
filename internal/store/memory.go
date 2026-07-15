package store

import (
	"context"
	"errors"
	"sync"
)

// MemoryStore is an in-process BlobStore backed by a mutex-guarded map.
// It mirrors TS's `abstract-blob-store()` (the in-memory default).
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryStore creates an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string][]byte)}
}

func (m *MemoryStore) Read(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.data[key]
	if !ok {
		return nil, ErrNotFound
	}
	// Return a copy so callers can't mutate the stored blob.
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

func (m *MemoryStore) SafeRead(ctx context.Context, key string) ([]byte, error) {
	b, err := m.Read(ctx, key)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return b, err
}

func (m *MemoryStore) Write(ctx context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store a copy to avoid aliasing caller's slice.
	out := make([]byte, len(data))
	copy(out, data)
	m.data[key] = out
	return nil
}

func (m *MemoryStore) ReadJSON(ctx context.Context, key string, target any) error {
	return readJSONFrom(ctx, m, key, target)
}

func (m *MemoryStore) WriteJSON(ctx context.Context, key string, value any) error {
	return writeJSONTo(ctx, m, key, value)
}
