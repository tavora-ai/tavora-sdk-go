package tavora

import (
	"context"
	"time"
)

// Project represents a Tavora space.
type Project struct {
	ID          string    `json:"id"`
	TeamID       string    `json:"team_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetProject returns the space associated with this client's API key.
func (c *Client) GetProject(ctx context.Context) (*Project, error) {
	var space Project
	if err := c.get(ctx, "/api/sdk/project", &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// SeedProjectResult reports whether the project already had agents
// before seeding, plus the resulting default agent identity.
type SeedProjectResult struct {
	AlreadySeeded bool   `json:"already_seeded"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
}

// SeedProject ensures the project has the platform-invariant
// default agent (one agent + v1.0.0 version + minimal eval suite).
// Idempotent: if any agent already exists, returns AlreadySeeded=true
// without mutating state. Equivalent to what signup runs after creating
// a brand-new project.
func (c *Client) SeedProject(ctx context.Context) (*SeedProjectResult, error) {
	var out SeedProjectResult
	if err := c.post(ctx, "/api/sdk/project/seed", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
