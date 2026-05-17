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
	ProjectID     string          `json:"project_id"`
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

// CreateSkill / DeleteSkill were removed 2026-05-17. Skill rows are
// authored by the code-first source-sync path; edit
// tavora/agents/<id>/skills/<name>.js (plus the matching .md prompt)
// and re-run `tavora dev`.

func (c *Client) ListSkills(ctx context.Context) ([]Skill, error) {
	var resp struct{ Skills []Skill `json:"skills"` }
	if err := c.get(ctx, "/api/sdk/skills", &resp); err != nil {
		return nil, err
	}
	return resp.Skills, nil
}

func (c *Client) GetSkill(ctx context.Context, id string) (*Skill, error) {
	var skill Skill
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/skills/%s", id), &skill); err != nil {
		return nil, err
	}
	return &skill, nil
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
