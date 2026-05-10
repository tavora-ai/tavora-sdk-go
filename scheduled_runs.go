package tavora

import (
	"context"
	"fmt"
	"time"
)

// ScheduledRun represents a scheduled agent execution.
type ScheduledRun struct {
	ID             string     `json:"id"`
	ProductID        string     `json:"product_id"`
	AgentSessionID string     `json:"agent_session_id"`
	Name           string     `json:"name"`
	CronExpression string     `json:"cron_expression"`
	Message        string     `json:"message"`
	Enabled        bool       `json:"enabled"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
	RunCount       int32      `json:"run_count"`
	LastError      string     `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateScheduledRunInput holds parameters for creating a scheduled run.
type CreateScheduledRunInput struct {
	AgentSessionID string `json:"agent_session_id"`
	Name           string `json:"name,omitempty"`
	CronExpression string `json:"cron_expression"`
	Message        string `json:"message"`
}

// ListScheduledRuns returns all scheduled runs in the space.
func (c *Client) ListScheduledRuns(ctx context.Context) ([]ScheduledRun, error) {
	var resp struct {
		ScheduledRuns []ScheduledRun `json:"scheduled_runs"`
	}
	if err := c.get(ctx, "/api/sdk/scheduled-runs", &resp); err != nil {
		return nil, err
	}
	return resp.ScheduledRuns, nil
}

// GetScheduledRun returns a scheduled run by ID.
func (c *Client) GetScheduledRun(ctx context.Context, id string) (*ScheduledRun, error) {
	var run ScheduledRun
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/scheduled-runs/%s", id), &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// CreateScheduledRun creates a new scheduled run.
func (c *Client) CreateScheduledRun(ctx context.Context, input CreateScheduledRunInput) (*ScheduledRun, error) {
	var run ScheduledRun
	if err := c.post(ctx, "/api/sdk/scheduled-runs", input, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

// DeleteScheduledRun deletes a scheduled run by ID.
func (c *Client) DeleteScheduledRun(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/scheduled-runs/%s", id))
}
