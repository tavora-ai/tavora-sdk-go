// Package tavora — memory stores (Stage 2 of the composable-primitives
// plan in tavora-go). Named, app-scoped, persistent key-value buckets
// the agent reads with `remember()` / `recall()` / `memories()` when its
// session is pinned via memory_store_id.
//
// Distinct from legacy per-session `agent_memory`: that path stays
// ephemeral when no store is pinned. A memory store survives session end
// and is reachable from any later session that pins the same id.
package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// MemoryStore is one named KV bucket inside an app.
type MemoryStore struct {
	ID        string          `json:"id"`
	AppID     string          `json:"app_id"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// MemoryEntry is one (key, value) row inside a store.
type MemoryEntry struct {
	MemoryStoreID string    `json:"memory_store_id"`
	Key           string    `json:"key"`
	Value         string    `json:"value"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateMemoryStoreInput holds the parameters for creating a memory store.
// Names are unique per (app, name).
type CreateMemoryStoreInput struct {
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// UpdateMemoryStoreInput updates store metadata. PATCH semantics:
// omitted Metadata preserves the current value.
type UpdateMemoryStoreInput struct {
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// CreateMemoryStore creates a named memory store. Returns 409 if a
// store with the same name already exists in the app.
func (c *Client) CreateMemoryStore(ctx context.Context, input CreateMemoryStoreInput) (*MemoryStore, error) {
	var store MemoryStore
	if err := c.post(ctx, "/api/sdk/memory-stores", input, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// ListMemoryStores returns every memory store in the app.
func (c *Client) ListMemoryStores(ctx context.Context) ([]MemoryStore, error) {
	var resp struct {
		MemoryStores []MemoryStore `json:"memory_stores"`
	}
	if err := c.get(ctx, "/api/sdk/memory-stores", &resp); err != nil {
		return nil, err
	}
	return resp.MemoryStores, nil
}

// GetMemoryStore returns one store by id.
func (c *Client) GetMemoryStore(ctx context.Context, id string) (*MemoryStore, error) {
	var store MemoryStore
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/memory-stores/%s", id), &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// UpdateMemoryStore patches a memory store's metadata.
func (c *Client) UpdateMemoryStore(ctx context.Context, id string, input UpdateMemoryStoreInput) (*MemoryStore, error) {
	var store MemoryStore
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/memory-stores/%s", id), input, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// DeleteMemoryStore deletes the store and (by FK cascade) every entry inside it.
func (c *Client) DeleteMemoryStore(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/memory-stores/%s", id))
}

// ListMemoryEntries returns every (key, value) row in a store.
func (c *Client) ListMemoryEntries(ctx context.Context, storeID string) ([]MemoryEntry, error) {
	var resp struct {
		Entries []MemoryEntry `json:"entries"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/memory-stores/%s/entries", storeID), &resp); err != nil {
		return nil, err
	}
	return resp.Entries, nil
}

// PutMemoryEntry upserts (key, value) — inserts when absent, overwrites
// when present. Keys are URL-escaped on the wire so callers can pass
// anything (including slashes) without breaking routing.
func (c *Client) PutMemoryEntry(ctx context.Context, storeID, key, value string) (*MemoryEntry, error) {
	var entry MemoryEntry
	path := fmt.Sprintf("/api/sdk/memory-stores/%s/entries/%s", storeID, url.PathEscape(key))
	if err := c.put(ctx, path, map[string]string{"value": value}, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// DeleteMemoryEntry removes one entry. Idempotent — 204 even when absent.
func (c *Client) DeleteMemoryEntry(ctx context.Context, storeID, key string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/memory-stores/%s/entries/%s", storeID, url.PathEscape(key)))
}
