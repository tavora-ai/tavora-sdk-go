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
	AppID     string          `json:"app_id"`
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

// GetSkillAuthoringGuide returns the canonical "how to write a Tavora
// skill module" guide as Markdown. The server generates the doc from
// live runtime introspection (registered primitives, reserved names),
// so the content stays in sync with the sandbox the skill will run in.
//
// Intended use: tooling fetches this and prints it or writes it to a
// file the user hands to an LLM (e.g. Claude Code) for skill authoring.
func (c *Client) GetSkillAuthoringGuide(ctx context.Context) (string, error) {
	resp, err := c.resty.R().
		SetContext(ctx).
		SetHeader("Accept", "text/markdown").
		Get("/api/sdk/skills/authoring-guide")
	if err != nil {
		return "", fmt.Errorf("tavora: request failed: %w", err)
	}
	if err := checkError(resp); err != nil {
		return "", err
	}
	return string(resp.Body()), nil
}
