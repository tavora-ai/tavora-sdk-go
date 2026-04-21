package tavora

import (
	"context"
	"fmt"
	"time"
)

// Store represents a document store within a space.
type Store struct {
	ID          string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateStoreInput holds the parameters for creating a store.
type CreateStoreInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateStoreInput holds the parameters for updating a store.
type UpdateStoreInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ListStores returns all stores in the space.
func (c *Client) ListStores(ctx context.Context) ([]Store, error) {
	var resp struct {
		Stores []Store `json:"stores"`
	}
	if err := c.get(ctx, "/api/sdk/stores", &resp); err != nil {
		return nil, err
	}
	return resp.Stores, nil
}

// GetStore returns a single store by ID.
func (c *Client) GetStore(ctx context.Context, id string) (*Store, error) {
	var resp struct {
		Store Store `json:"store"`
	}
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/stores/%s", id), &resp); err != nil {
		return nil, err
	}
	return &resp.Store, nil
}

// CreateStore creates a new store.
func (c *Client) CreateStore(ctx context.Context, input CreateStoreInput) (*Store, error) {
	var store Store
	if err := c.post(ctx, "/api/sdk/stores", input, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// UpdateStore updates a store by ID.
func (c *Client) UpdateStore(ctx context.Context, id string, input UpdateStoreInput) (*Store, error) {
	var store Store
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/stores/%s", id), input, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// DeleteStore deletes a store by ID.
func (c *Client) DeleteStore(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/stores/%s", id))
}
