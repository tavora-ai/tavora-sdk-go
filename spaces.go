package tavora

import (
	"context"
	"time"
)

// Workspace represents a Tavora space.
type Workspace struct {
	ID          string    `json:"id"`
	TeamID       string    `json:"team_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetWorkspace returns the space associated with this client's API key.
func (c *Client) GetWorkspace(ctx context.Context) (*Workspace, error) {
	var space Workspace
	if err := c.get(ctx, "/api/sdk/space", &space); err != nil {
		return nil, err
	}
	return &space, nil
}

// SeedWorkspaceResult reports whether the workspace already had agents
// before seeding, plus the resulting default agent identity.
type SeedWorkspaceResult struct {
	AlreadySeeded bool   `json:"already_seeded"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
}

// SeedWorkspace ensures the workspace has the platform-invariant
// default agent (one agent + v1.0.0 version + minimal eval suite).
// Idempotent: if any agent already exists, returns AlreadySeeded=true
// without mutating state. Equivalent to what signup runs after creating
// a brand-new workspace.
func (c *Client) SeedWorkspace(ctx context.Context) (*SeedWorkspaceResult, error) {
	var out SeedWorkspaceResult
	if err := c.post(ctx, "/api/sdk/workspace/seed", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
