package tavora

import (
	"context"
	"fmt"
	"time"
)

// EvalSuite is a named grouping of eval cases for advisory eval runs.
// Pre-agent-simplification (PR4) suites also gated agent promotion;
// that's gone — suites now just describe "the set of cases to run."
type EvalSuite struct {
	ID              string    `json:"id"`
	AppID           string    `json:"app_id"`
	AgentID         *string   `json:"agent_id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Threshold       float32   `json:"threshold"`
	ActiveVersionID *string   `json:"active_version_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// EvalSuiteVersion is an immutable snapshot of an EvalSuite's case membership.
type EvalSuiteVersion struct {
	ID        string    `json:"id"`
	SuiteID   string    `json:"suite_id"`
	Semver    string    `json:"semver"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Input types ---

type CreateSuiteInput struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Threshold   float32 `json:"threshold,omitempty"` // 0–1; 0.8 default if zero
	AgentID     string  `json:"agent_id,omitempty"`  // optional: attach at create time
}

type NewSuiteVersionInput struct {
	// CaseIDs is the new membership snapshot. Nil (not empty) means "inherit
	// from the suite's active version" — the common bump-version path.
	CaseIDs []string `json:"case_ids,omitempty"`
}

// --- Suite methods ---

func (c *Client) CreateSuite(ctx context.Context, input CreateSuiteInput) (*EvalSuite, error) {
	var out EvalSuite
	if err := c.post(ctx, "/api/sdk/eval-suites", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListSuites(ctx context.Context) ([]EvalSuite, error) {
	var out []EvalSuite
	if err := c.get(ctx, "/api/sdk/eval-suites", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetSuite(ctx context.Context, suiteID string) (*EvalSuite, error) {
	var out EvalSuite
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/eval-suites/%s", suiteID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteSuite(ctx context.Context, suiteID string) error {
	return c.delete(ctx, fmt.Sprintf("/api/sdk/eval-suites/%s", suiteID))
}

// NewSuiteVersion freezes the suite's case membership into an immutable
// version. Omit CaseIDs to inherit the suite's current active-version
// membership.
func (c *Client) NewSuiteVersion(ctx context.Context, suiteID string, input NewSuiteVersionInput) (*EvalSuiteVersion, error) {
	var out EvalSuiteVersion
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/eval-suites/%s/versions", suiteID), input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
