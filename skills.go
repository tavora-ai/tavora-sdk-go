package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Skill represents a custom tool definition.
type Skill struct {
	ID          string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Type        string          `json:"type"`
	Prompt      string          `json:"prompt"`
	Config      json.RawMessage `json:"config"`
	Parameters  json.RawMessage `json:"parameters"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateSkillInput struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        string          `json:"type,omitempty"`
	Prompt      string          `json:"prompt,omitempty"`
	Config      json.RawMessage `json:"config,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

func (c *Client) ListSkills(ctx context.Context) ([]Skill, error) {
	var resp struct{ Skills []Skill `json:"skills"` }
	if err := c.get(ctx, "/api/sdk/skills", &resp); err != nil {
		return nil, err
	}
	return resp.Skills, nil
}

func (c *Client) CreateSkill(ctx context.Context, input CreateSkillInput) (*Skill, error) {
	var skill Skill
	if err := c.post(ctx, "/api/sdk/skills", input, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

func (c *Client) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var skill Skill
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/skills/%s", id), &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

func (c *Client) DeleteSkill(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/skills/%s", id))
}
