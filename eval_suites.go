package tavora

import (
	"context"
	"fmt"
	"time"
)

// EvalSuite is a named grouping of eval cases. Pre-agent-simplification
// suites gated agent promotion; today they describe "the set of cases
// to run." One suite per agent at the schema level.
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

// CreateSuite / DeleteSuite / NewSuiteVersion were removed
// 2026-05-17 alongside the broader eval dual-writer cleanup. Suites
// and their versions are minted exclusively by `tavora dev` from
// the agent's tavora/agents/<id>/evals/*.json files; the SDK
// surface stays read-only so the eval_suites / eval_suite_versions
// tables have a single writer.

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

// ListSuiteVersions returns the immutable version snapshots minted by
// source-sync each time the eval-case set changes.
func (c *Client) ListSuiteVersions(ctx context.Context, suiteID string) ([]EvalSuiteVersion, error) {
	var out []EvalSuiteVersion
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/eval-suites/%s/versions", suiteID), &out); err != nil {
		return nil, err
	}
	return out, nil
}
