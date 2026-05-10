package tavora

import (
	"context"
	"time"
)

// Product represents a Tavora space.
type Product struct {
	ID          string    `json:"id"`
	TeamID       string    `json:"team_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetProduct returns the space associated with this client's API key.
func (c *Client) GetProduct(ctx context.Context) (*Product, error) {
	var space Product
	if err := c.get(ctx, "/api/sdk/product", &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// SeedProductResult reports whether the product already had agents
// before seeding, plus the resulting default agent identity.
type SeedProductResult struct {
	AlreadySeeded bool   `json:"already_seeded"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
}

// SeedProduct ensures the product has the platform-invariant
// default agent (one agent + v1.0.0 version + minimal eval suite).
// Idempotent: if any agent already exists, returns AlreadySeeded=true
// without mutating state. Equivalent to what signup runs after creating
// a brand-new product.
func (c *Client) SeedProduct(ctx context.Context) (*SeedProductResult, error) {
	var out SeedProductResult
	if err := c.post(ctx, "/api/sdk/product/seed", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
