package tavora

import (
	"context"
	"fmt"
	"time"
)

// EvalSuite is a Phase-12 primitive: a named grouping of eval cases that
// gates agent version promotions.
type EvalSuite struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspace_id"`
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

// AgentPromotion is the state-machine row gating a deployment on evals.
type AgentPromotion struct {
	ID             string     `json:"id"`
	VersionID      string     `json:"version_id"`
	TargetType     string     `json:"target_type"`
	TargetRef      string     `json:"target_ref"`
	EvalRunID      *string    `json:"eval_run_id"`
	Status         string     `json:"status"` // pending_eval | pending_approval | approved | rejected | deployed | failed_eval
	ApproverUserID *string    `json:"approver_user_id"`
	DecidedAt      *time.Time `json:"decided_at"`
	Reason         string     `json:"reason"`
	ProposedBy     string     `json:"proposed_by"`
	ProposedAt     time.Time  `json:"proposed_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
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

type ProposePromotionInput struct {
	VersionID  string `json:"version_id"`
	TargetType string `json:"target_type,omitempty"` // defaults to "api"
	TargetRef  string `json:"target_ref,omitempty"`
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

// --- Promotion methods ---

// ProposePromotion proposes pinning an agent version at a target. The
// returned promotion starts in pending_eval; the server-side eval
// runner drives it to pending_approval (or failed_eval) automatically
// once the attached suite finishes.
func (c *Client) ProposePromotion(ctx context.Context, input ProposePromotionInput) (*AgentPromotion, error) {
	var out AgentPromotion
	if err := c.post(ctx, "/api/sdk/promotions", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ApprovePromotion(ctx context.Context, promotionID string) (*AgentPromotion, error) {
	var out AgentPromotion
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/promotions/%s/approve", promotionID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RejectPromotion requires a non-empty reason.
func (c *Client) RejectPromotion(ctx context.Context, promotionID, reason string) (*AgentPromotion, error) {
	body := map[string]string{"reason": reason}
	var out AgentPromotion
	if err := c.post(ctx, fmt.Sprintf("/api/sdk/promotions/%s/reject", promotionID), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPromotion(ctx context.Context, promotionID string) (*AgentPromotion, error) {
	var out AgentPromotion
	if err := c.get(ctx, fmt.Sprintf("/api/sdk/promotions/%s", promotionID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListPendingPromotions(ctx context.Context) ([]AgentPromotion, error) {
	var out []AgentPromotion
	if err := c.get(ctx, "/api/sdk/promotions/pending", &out); err != nil {
		return nil, err
	}
	return out, nil
}
