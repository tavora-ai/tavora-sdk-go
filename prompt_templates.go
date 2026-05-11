package tavora

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PromptTemplate represents a reusable system prompt.
type PromptTemplate struct {
	ID        string          `json:"id"`
	AppID   string          `json:"app_id"`
	Name      string          `json:"name"`
	Content   string          `json:"content"`
	Variables json.RawMessage `json:"variables"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// CreatePromptTemplateInput holds parameters for creating a prompt template.
type CreatePromptTemplateInput struct {
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Variables []string `json:"variables,omitempty"`
}

// UpdatePromptTemplateInput holds parameters for updating a prompt template.
type UpdatePromptTemplateInput struct {
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Variables []string `json:"variables,omitempty"`
}

// ListPromptTemplates returns all prompt templates in the space.
func (c *Client) ListPromptTemplates(ctx context.Context) ([]PromptTemplate, error) {
	var resp struct {
		Templates []PromptTemplate `json:"templates"`
	}
	if err := c.get(ctx, "/api/sdk/prompt-templates", &resp); err != nil {
		return nil, err
	}
	return resp.Templates, nil
}

// GetPromptTemplate returns a prompt template by ID.
func (c *Client) GetPromptTemplate(ctx context.Context, id string) (*PromptTemplate, error) {
	var tmpl PromptTemplate
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/prompt-templates/%s", id), &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// CreatePromptTemplate creates a new prompt template.
func (c *Client) CreatePromptTemplate(ctx context.Context, input CreatePromptTemplateInput) (*PromptTemplate, error) {
	var tmpl PromptTemplate
	if err := c.post(ctx, "/api/sdk/prompt-templates", input, &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// UpdatePromptTemplate updates a prompt template by ID.
func (c *Client) UpdatePromptTemplate(ctx context.Context, id string, input UpdatePromptTemplateInput) (*PromptTemplate, error) {
	var tmpl PromptTemplate
	if err := c.patch(ctx, fmt.Sprintf("/api/sdk/prompt-templates/%s", id), input, &tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// DeletePromptTemplate deletes a prompt template by ID.
func (c *Client) DeletePromptTemplate(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/prompt-templates/%s", id))
}
