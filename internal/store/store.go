// Package store provides a blob storage abstraction for conveyor's drafts
// and feature-flags, mirroring the original TS AsyncBlobStore (abstract-blob-store
// for memory, s3-blob-store for S3). The BlobStore interface is shape-agnostic:
// callers store arbitrary byte blobs, with JSON convenience helpers on top.
package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/steemit/conveyor/internal/config"
)

// ErrNotFound is returned by Read when the key does not exist. SafeRead
// swallows this error and returns (nil, nil) instead.
var ErrNotFound = errors.New("blob not found")

// BlobStore is the key-value storage interface used by drafts and feature-flags.
type BlobStore interface {
	// Read returns the raw bytes for key, or ErrNotFound if the key does not
	// exist.
	Read(ctx context.Context, key string) ([]byte, error)

	// SafeRead is like Read but returns (nil, nil) when the key is missing,
	// mirroring TS AsyncBlobStore.safeRead.
	SafeRead(ctx context.Context, key string) ([]byte, error)

	// Write stores data under key, overwriting any previous value.
	Write(ctx context.Context, key string, data []byte) error

	// ReadJSON reads the key and JSON-unmarshals into target.
	//
	// IMPORTANT: when the key does not exist, ReadJSON returns a nil error
	// and does NOT modify target. The caller MUST pre-initialise target to
	// the desired default value. This mirrors the TS pattern
	// `store.readJSON(key) || []` / `|| {}` — there `readJSON` returns
	// undefined and the `||` supplies the default; in Go the caller supplies
	// the default before calling. Example:
	//
	//   drafts := []any{}                  // default
	//   _ = store.ReadJSON(ctx, key, &drafts) // key missing → drafts stays []
	ReadJSON(ctx context.Context, key string, target any) error

	// WriteJSON JSON-marshals value and stores it under key.
	WriteJSON(ctx context.Context, key string, value any) error
}

// NewStore creates a BlobStore from config. type "memory" → in-memory map;
// type "s3" → AWS S3. Any other type returns an error.
func NewStore(ctx context.Context, cfg config.StorageConfig) (BlobStore, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryStore(), nil
	case "s3":
		return NewS3Store(ctx, cfg.S3Bucket)
	default:
		return nil, errors.New("invalid storage type: " + cfg.Type)
	}
}

// --- JSON helpers usable by any BlobStore implementation ---

func readJSONFrom(ctx context.Context, s BlobStore, key string, target any) error {
	data, err := s.SafeRead(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		// Key not found — do not modify target (caller's default applies).
		return nil
	}
	return json.Unmarshal(data, target)
}

func writeJSONTo(ctx context.Context, s BlobStore, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.Write(ctx, key, data)
}
