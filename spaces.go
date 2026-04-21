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
