package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Index is an app-scoped container of RAG-indexed documents — what other
// ecosystems call a "vector store." Naming history: stores → indexes
// (tavora-go migration 00047). The Collections + Files surfaces that
// shared this naming taxonomy were retired by the 2026-05-11 positioning
// rewrite — Indexes is the sole retrieval primitive Tavora owns.
type Index struct {
	ID          string          `json:"id"`
	AppID       string          `json:"app_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// CreateIndexInput holds the parameters for creating an index.
type CreateIndexInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// UpdateIndexInput holds the parameters for updating an index. PATCH
// semantics: omitted Metadata preserves the current value; pass `json.RawMessage("{}")`
// to clear.
type UpdateIndexInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// ListIndexes returns all indexes in the app.
func (c *Client) ListIndexes(ctx context.Context) ([]Index, error) {
	var resp struct {
		Indexes []Index `json:"indexes"`
	}
	if err := c.get(ctx, "/api/sdk/indexes", &resp); err != nil {
		return nil, err
	}
	return resp.Indexes, nil
}

// GetIndex returns a single index by ID.
func (c *Client) GetIndex(ctx context.Context, id string) (*Index, error) {
	var resp struct {
		Index Index `json:"index"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/indexes/%s", id), &resp); err != nil {
		return nil, err
	}
	return &resp.Index, nil
}

// CreateIndex creates a new index.
func (c *Client) CreateIndex(ctx context.Context, input CreateIndexInput) (*Index, error) {
	var idx Index
	if err := c.post(ctx, "/api/sdk/indexes", input, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// UpdateIndex updates an index by ID.
func (c *Client) UpdateIndex(ctx context.Context, id string, input UpdateIndexInput) (*Index, error) {
	var idx Index
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/indexes/%s", id), input, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// DeleteIndex deletes an index by ID.
func (c *Client) DeleteIndex(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/indexes/%s", id))
}
