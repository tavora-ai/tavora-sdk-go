package tavora

import (
	"context"
	"time"
)

// App represents a Tavora space.
type App struct {
	ID          string    `json:"id"`
	TeamID       string    `json:"team_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetApp returns the space associated with this client's API key.
func (c *Client) GetApp(ctx context.Context) (*App, error) {
	var space App
	if err := c.get(ctx, "/api/sdk/app", &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// SeedAppResult reports whether the app already had agents
// before seeding, plus the resulting default agent identity.
type SeedAppResult struct {
	AlreadySeeded bool   `json:"already_seeded"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
}

// SeedApp ensures the app has the platform-invariant
// default agent (one agent + v1.0.0 version + minimal eval suite).
// Idempotent: if any agent already exists, returns AlreadySeeded=true
// without mutating state. Equivalent to what signup runs after creating
// a brand-new app.
func (c *Client) SeedApp(ctx context.Context) (*SeedAppResult, error) {
	var out SeedAppResult
	if err := c.post(ctx, "/api/sdk/app/seed", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
